// Package gxcli implements the gx command line tool.
package gxcli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/alternayte/gx/internal/analyze"
	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/devserver"
	"github.com/alternayte/gx/internal/lsp"
)

// Main runs the gx command with the given arguments and returns an exit code.
func Main(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "fmt":
		return runFmt(args[1:])
	case "check":
		return runCheck(args[1:])
	case "generate":
		return runGenerate(args[1:])
	case "build":
		return runBuild(args[1:])
	case "dev":
		return runDev(args[1:])
	case "routes":
		return runRoutes(args[1:])
	case "lsp":
		return runLSP(args[1:])
	case "lint":
		return runLint(args[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "gx: unknown command %q\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: gx <command> [arguments]

Commands:
  fmt       format .gx files in place, or stdin when no path is given
  check     check a module and fail on stale generated code
  generate  write the generated Go files of a module
  build     generate the module and build its app binary
  dev       run the app with rebuild, restart, morph and the error overlay
  routes    print the routes of a module, with --json for machine output
  lsp       run the language server on stdio
  lint      run go vet and the Gx analyzers on a module
`)
}

func runFmt(args []string) int {
	fs := flag.NewFlagSet("gx fmt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.Bool("check", false, "exit non-zero when a file is not formatted")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		return fmtStdin(*check)
	}
	status := 0
	for _, path := range paths {
		if code := fmtFile(path, *check); code != 0 {
			status = code
		}
	}
	return status
}

func fmtStdin(check bool) int {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx fmt: %v\n", err)
		return 1
	}
	out, diags := compiler.FormatSource("", src)
	if len(diags) > 0 {
		printDiags(diags)
		return 1
	}
	return writeResult("stdin", src, out, check, "")
}

func fmtFile(path string, check bool) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx fmt: %v\n", err)
		return 1
	}
	out, diags := compiler.FormatSource(path, src)
	if len(diags) > 0 {
		printDiags(diags)
		return 1
	}
	return writeResult(path, src, out, check, path)
}

func writeResult(name string, src, out []byte, check bool, path string) int {
	if check {
		if !bytes.Equal(src, out) {
			fmt.Fprintf(os.Stderr, "gx fmt: %s is not formatted\n", name)
			return 1
		}
		return 0
	}
	if path == "" {
		if _, err := os.Stdout.Write(out); err != nil {
			fmt.Fprintf(os.Stderr, "gx fmt: %v\n", err)
			return 1
		}
		return 0
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, out, mode); err != nil {
		fmt.Fprintf(os.Stderr, "gx fmt: %v\n", err)
		return 1
	}
	return 0
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("gx check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print machine-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	diags := compiler.Check(dir)
	diags = append(diags, compiler.Stale(dir)...)
	if *asJSON {
		return printJSON(diags)
	}
	if len(diags) == 0 {
		return 0
	}
	printDiags(diags)
	return 1
}

type jsonDiagnostic struct {
	Code    string `json:"code"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
	Doc     string `json:"doc"`
}

func printJSON(diags []compiler.Diagnostic) int {
	out := make([]jsonDiagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, jsonDiagnostic{
			Code:    d.Code,
			File:    d.File,
			Line:    d.Line,
			Column:  d.Col,
			Message: d.Msg,
			Fix:     d.Fix,
			Doc:     d.Doc(),
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx check: %v\n", err)
		return 1
	}
	fmt.Println(string(data))
	if len(diags) > 0 {
		return 1
	}
	return 0
}

func runRoutes(args []string) int {
	fs := flag.NewFlagSet("gx routes", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print machine-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	reports, diags := compiler.Routes(dir)
	if len(diags) > 0 {
		printDiags(diags)
		return 1
	}
	if *asJSON {
		data, err := json.MarshalIndent(reports, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "gx routes: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
		return 0
	}
	for _, r := range reports {
		line := r.Method + " " + r.Pattern + "  " + r.Type
		if r.Page != "" {
			line += "  page=" + r.Page
		}
		if len(r.Fields) > 0 {
			line += "  fields=" + strings.Join(r.Fields, ",")
		}
		if r.Prefix != "" {
			line += "  prefix=" + r.Prefix
		}
		if len(r.Layouts) > 0 {
			line += "  layouts=" + strings.Join(r.Layouts, ",")
		}
		if len(r.Middleware) > 0 {
			line += "  middleware=" + strings.Join(r.Middleware, ",")
		}
		fmt.Println(line)
	}
	return 0
}

func runBuild(args []string) int {
	fs := flag.NewFlagSet("gx build", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("o", "", "output binary")
	main := fs.String("main", "", "app main package")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	if *main == "" {
		var err error
		*main, err = devserver.DetectMain(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gx build: %v\n", err)
			return 1
		}
	}
	if files, diags := compiler.Generate(dir); len(diags) > 0 {
		printDiags(diags)
		return 1
	} else {
		for path, src := range files {
			if err := os.WriteFile(path, src, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "gx build: %v\n", err)
				return 1
			}
		}
	}
	bin := *out
	if bin == "" {
		bin = filepath.Join(dir, "app")
	}
	cmd := exec.Command("go", "build", "-o", bin, *main)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "gx build: %v\n", err)
		return 1
	}
	return 0
}

func runLint(args []string) int {
	fs := flag.NewFlagSet("gx lint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	vet := exec.Command("go", "vet", "./...")
	vet.Dir = dir
	vet.Stdout, vet.Stderr = os.Stdout, os.Stderr
	vetErr := vet.Run()
	findings, err := analyze.Lint(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx lint: %v\n", err)
		return 1
	}
	for _, f := range findings {
		fmt.Fprintf(os.Stderr, "%s:%d:%d: %s: %s\n", f.File, f.Line, f.Col, f.Code, f.Message)
	}
	if vetErr != nil || len(findings) > 0 {
		return 1
	}
	return 0
}

func runLSP(args []string) int {
	fs := flag.NewFlagSet("gx lsp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	root := fs.String("root", "", "module root (default: current directory)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := *root
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "gx lsp: %v\n", err)
			return 1
		}
	}
	if err := lsp.Serve(os.Stdin, os.Stdout, lsp.Options{Root: dir, Log: os.Stderr}); err != nil {
		fmt.Fprintf(os.Stderr, "gx lsp: %v\n", err)
		return 1
	}
	return 0
}

func runDev(args []string) int {
	fs := flag.NewFlagSet("gx dev", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	addr := fs.String("addr", "127.0.0.1:3333", "dev proxy address")
	main := fs.String("main", "", "app main package")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := devserver.Run(ctx, devserver.Options{Dir: dir, Main: *main, Addr: *addr, Log: os.Stdout}); err != nil {
		fmt.Fprintf(os.Stderr, "gx dev: %v\n", err)
		return 1
	}
	return 0
}

func runGenerate(args []string) int {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		printDiags(diags)
		return 1
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := os.WriteFile(path, files[path], 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "gx generate: %v\n", err)
			return 1
		}
	}
	return 0
}

func printDiags(diags []compiler.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintln(os.Stderr, d.String())
	}
}
