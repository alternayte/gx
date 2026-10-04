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
	"github.com/alternayte/gx/internal/exporter"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/gxstyles"
	"github.com/alternayte/gx/internal/icons"
	"github.com/alternayte/gx/internal/lsp"
	pagefindpkg "github.com/alternayte/gx/internal/pagefind"
	"github.com/alternayte/gx/internal/registry"
	"github.com/alternayte/gx/internal/starlight"
	tailwindpkg "github.com/alternayte/gx/internal/tailwind"
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
	case "icons":
		return runIcons(args[1:])
	case "vendor":
		return runVendor(args[1:])
	case "export":
		return runExport(args[1:])
	case "import":
		return runImport(args[1:])
	case "registry":
		return runRegistry(args[1:])
	case "add":
		return runAdd(args[1:])
	case "diff":
		return runDiff(args[1:])
	case "update":
		return runUpdate(args[1:])
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
  icons pin pin an icon set and generate one .gx component per icon
  vendor    store the pinned downloads in .gx/vendor for offline builds
  export    render every GET page to static files with --out <dir>
  import    convert another tool: gx import starlight --out <dir> <src>
  registry  build a publishable component registry
  add       install a registry item and its dependencies
  diff      show local, base and upstream changes of an item
  update    merge the current registry version into an item
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
	external := fs.Bool("external-links", false, "request every external content link")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	diags := compiler.CheckWith(dir, compiler.CheckOptions{ExternalLinks: *external})
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
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "gx build: %v\n", err)
				return 1
			}
			if err := os.WriteFile(path, src, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "gx build: %v\n", err)
				return 1
			}
		}
	}
	if _, err := gxstyles.Build(context.Background(), dir, true); err != nil {
		fmt.Fprintf(os.Stderr, "gx build: %v\n", err)
		return 1
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

// runExport renders a module to static files (REQ-EXP-01).
func runExport(args []string) int {
	fs := flag.NewFlagSet("gx export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "dist", "output directory")
	mainPkg := fs.String("main", "", "main package path")
	siteURL := fs.String("site", "", "canonical site URL (overrides [site] url)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	res, err := exporter.Export(ctx, exporter.Options{Dir: dir, Out: *out, Main: *mainPkg, SiteURL: *siteURL, Log: os.Stdout})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx export: %v\n", err)
		return 1
	}
	fmt.Printf("exported %d pages to %s\n", len(res.Paths), *out)
	return 0
}

// splitRef splits an item reference into its registry namespace, item name
// and version, for example "@acme/button@0.2.0" (REQ-REG-04).
func splitRef(ref string) (namespace, name, version string) {
	name = ref
	if i := strings.LastIndex(ref, "@"); i > 0 {
		name, version = ref[:i], ref[i+1:]
	}
	if strings.HasPrefix(name, "@") {
		if i := strings.Index(name, "/"); i > 1 {
			namespace, name = name[1:i], name[i+1:]
		}
	}
	return namespace, name, version
}

// registryInstaller builds the installer of an app from gx.toml and the
// command flags (REQ-REG-02, REQ-REG-04). A namespace selects a named
// registry; --registry overrides every source.
func registryInstaller(root, source, dir, namespace string) (registry.Installer, int) {
	cfg, err := gxconfig.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx registry: %v\n", err)
		return registry.Installer{}, 1
	}
	src := source
	var headers []string
	if src == "" && namespace != "" {
		named, ok := cfg.Registries[namespace]
		if !ok {
			fmt.Fprintf(os.Stderr, "gx registry: unknown registry @%s\n", namespace)
			return registry.Installer{}, 1
		}
		src = named.URL
		headers = named.Headers
	}
	if src == "" {
		src = cfg.Registry.URL
	}
	if src == "" {
		if official, ok := cfg.Registries["official"]; ok {
			src = official.URL
			headers = official.Headers
		}
	}
	if src == "" {
		fmt.Fprintln(os.Stderr, "gx registry: no registry; set [registry] url in gx.toml or pass --registry")
		return registry.Installer{}, 1
	}
	installDir := dir
	if installDir == "" {
		installDir = cfg.Registry.Dir
	}
	return registry.Installer{Root: root, Source: src, Dir: installDir, Headers: headers}, 0
}

// runAdd installs a registry item and its dependencies (REQ-REG-02).
func runAdd(args []string) int {
	fs := flag.NewFlagSet("gx add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	source := fs.String("registry", "", "registry URL or directory (overrides [registry] url)")
	dir := fs.String("dir", "", "install directory (overrides [registry] dir)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx add [--registry <url>] [--dir <dir>] <item>[@version] [app]")
		return 2
	}
	namespace, name, version := splitRef(rest[0])
	root := "."
	if len(rest) > 1 {
		root = rest[1]
	}
	inst, code := registryInstaller(root, *source, *dir, namespace)
	if code != 0 {
		return code
	}
	items, err := inst.Add(name, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx add: %v\n", err)
		return 1
	}
	for _, item := range items {
		fmt.Printf("added %s@%s: %s\n", item.Name, item.Version, item.Description)
	}
	return 0
}

// runDiff shows local, base and upstream changes of an installed item
// (REQ-REG-03).
func runDiff(args []string) int {
	fs := flag.NewFlagSet("gx diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	source := fs.String("registry", "", "registry URL or directory (overrides [registry] url)")
	dir := fs.String("dir", "", "install directory (overrides [registry] dir)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx diff [--registry <url>] <item> [app]")
		return 2
	}
	namespace, name, _ := splitRef(rest[0])
	root := "."
	if len(rest) > 1 {
		root = rest[1]
	}
	inst, code := registryInstaller(root, *source, *dir, namespace)
	if code != 0 {
		return code
	}
	changes, err := inst.Diff(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx diff: %v\n", err)
		return 1
	}
	for _, change := range changes {
		fmt.Printf("%-16s %s\n", change.Status, change.Target)
	}
	return 0
}

// runUpdate merges the current registry version into an installed item
// (REQ-REG-03).
func runUpdate(args []string) int {
	fs := flag.NewFlagSet("gx update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	source := fs.String("registry", "", "registry URL or directory (overrides [registry] url)")
	dir := fs.String("dir", "", "install directory (overrides [registry] dir)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx update [--registry <url>] <item>[@version] [app]")
		return 2
	}
	namespace, name, version := splitRef(rest[0])
	root := "."
	if len(rest) > 1 {
		root = rest[1]
	}
	inst, code := registryInstaller(root, *source, *dir, namespace)
	if code != 0 {
		return code
	}
	changes, conflict, err := inst.Update(name, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx update: %v\n", err)
		return 1
	}
	for _, change := range changes {
		fmt.Printf("%-16s %s\n", change.Status, change.Target)
	}
	if conflict {
		fmt.Fprintln(os.Stderr, "gx update: conflict markers written; resolve them in place")
		return 1
	}
	return 0
}

// runRegistry builds a publishable registry (REQ-REG-01).
func runRegistry(args []string) int {
	if len(args) == 0 || args[0] != "build" {
		fmt.Fprintln(os.Stderr, "usage: gx registry build [--out <dir>] <src>")
		return 2
	}
	fs := flag.NewFlagSet("gx registry build", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "", "output directory (default the source directory)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx registry build [--out <dir>] <src>")
		return 2
	}
	src := rest[0]
	dest := *out
	if dest == "" {
		dest = src
	}
	index, err := registry.Build(src, dest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx registry build: %v\n", err)
		return 1
	}
	fmt.Printf("registry: %d items\n", len(index.Items))
	return 0
}

// runImport converts another tool's project into a Gx app (REQ-CNT-13).
func runImport(args []string) int {
	if len(args) == 0 || args[0] != "starlight" {
		fmt.Fprintln(os.Stderr, "usage: gx import starlight --out <dir> <src>")
		return 2
	}
	fs := flag.NewFlagSet("gx import starlight", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "content/docs", "Gx content directory")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx import starlight --out <dir> <src>")
		return 2
	}
	report, err := starlight.Convert(starlight.Options{Src: rest[0], Out: *out})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx import starlight: %v\n", err)
		return 1
	}
	fmt.Printf("converted %d pages\n", report.Converted)
	for _, note := range report.Notes {
		fmt.Println("note:", note)
	}
	return 0
}

func runVendor(args []string) int {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	path, err := (&tailwindpkg.Manager{Root: dir}).Vendor(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx vendor: tailwind: %v\n", err)
		return 1
	}
	fmt.Println("vendored", path)
	pfPath, err := (&pagefindpkg.Manager{Root: dir}).Vendor(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx vendor: pagefind: %v\n", err)
		return 1
	}
	fmt.Println("vendored", pfPath)
	if !pagefindpkg.HasLock(dir) {
		if err := pagefindpkg.SaveLock(dir, pagefindpkg.DefaultLock().Pagefind); err != nil {
			fmt.Fprintf(os.Stderr, "gx vendor: pagefind lock: %v\n", err)
			return 1
		}
	}
	pins, err := icons.Pinned(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx vendor: %v\n", err)
		return 1
	}
	for set, entry := range pins {
		path, err := icons.Vendor(ctx, icons.Options{Dir: dir, Set: set, Version: entry.Version})
		if err != nil {
			fmt.Fprintf(os.Stderr, "gx vendor: icons %s: %v\n", set, err)
			return 1
		}
		fmt.Println("vendored", path)
	}
	return 0
}

func runIcons(args []string) int {
	if len(args) == 0 || args[0] != "pin" {
		fmt.Fprintln(os.Stderr, "usage: gx icons pin <set>@<version> [dir]")
		return 2
	}
	fs := flag.NewFlagSet("gx icons pin", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "", "output directory (default ui/icons/<set>)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx icons pin <set>@<version> [dir]")
		return 2
	}
	set, version, ok := strings.Cut(rest[0], "@")
	if !ok || set == "" || version == "" {
		fmt.Fprintln(os.Stderr, "usage: gx icons pin <set>@<version> [dir]")
		return 2
	}
	dir := "."
	if len(rest) > 1 {
		dir = rest[1]
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	err := icons.Pin(ctx, icons.Options{Dir: dir, Set: set, Version: version, Out: *out})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx icons: %v\n", err)
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
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "gx generate: %v\n", err)
			return 1
		}
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
