// Package gxcli implements the gx command line tool.
package gxcli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/alternayte/gx/internal/compiler"
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
