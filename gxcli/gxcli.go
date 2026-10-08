// Package gxcli implements the gx command line tool.
package gxcli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/alternayte/gx/internal/analyze"
	"github.com/alternayte/gx/internal/appmodel"
	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/devserver"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/exporter"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/gxstyles"
	"github.com/alternayte/gx/internal/icons"
	"github.com/alternayte/gx/internal/islands"
	"github.com/alternayte/gx/internal/jspin"
	"github.com/alternayte/gx/internal/lsp"
	"github.com/alternayte/gx/internal/mcpserver"
	pagefindpkg "github.com/alternayte/gx/internal/pagefind"
	"github.com/alternayte/gx/internal/registry"
	"github.com/alternayte/gx/internal/scaffold"
	"github.com/alternayte/gx/internal/starlight"
	tailwindpkg "github.com/alternayte/gx/internal/tailwind"
	"github.com/alternayte/gx/internal/tscheck"
	"github.com/alternayte/gx/internal/wcpin"
	"github.com/alternayte/gx/internal/widgetelement"
)

// Version is the Gx release this command line tool belongs to. `gx init`
// writes it into the go.mod of a new app.
const Version = "0.2.1"

// Main runs the gx command with the given arguments and returns an exit code.
func Main(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "new":
		return runNew(args[1:])
	case "agents":
		return runAgents(args[1:])
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
	case "describe":
		return runDescribe(args[1:])
	case "lsp":
		return runLSP(args[1:])
	case "mcp":
		return runMCP(args[1:])
	case "lint":
		return runLint(args[1:])
	case "icons":
		return runIcons(args[1:])
	case "pin":
		return runPin(args[1:])
	case "wc":
		return runWC(args[1:])
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
  init      write a new app: gx init [--template app|docs] [--module <path>] [dir]
  new       add typed code: gx new page|action|form|component|slice <name>
  agents    refresh the managed section of AGENTS.md with --update
  fmt       format .gx files in place, or stdin when no path is given
  check     check a module and fail on stale generated code, with --json
  generate  write the generated Go files of a module
  build     generate the module and build its app binary
  dev       run the app with rebuild, restart, morph and the error overlay
  routes    print the routes of a module, with --json for machine output
  describe  print the app model, with --json for machine output
  lsp       run the language server on stdio
  mcp       run the dev MCP server on stdio, for a coding agent
  lint      run go vet and the Gx analyzers on a module, with --json
  icons pin pin an icon set and generate one .gx component per icon
  pin       vendor an npm package for the islands: gx pin <pkg>@<version>
  wc pin    import the web components of an npm package as typed tags
  wc build  write the element file and the types of each widget for a host page
  vendor    store the pinned downloads in .gx/vendor for offline builds
  export    render every GET page to static files with --out <dir>
  import    convert another tool: gx import starlight --out <dir> <src>
  registry  build or lint a publishable component registry
  add       install a registry item and its dependencies
  diff      show local, base and upstream changes of an item
  update    merge the current registry version into an item
`)
}

// runInit writes a new app (REQ-DEV-10).
func runInit(args []string) int {
	fs := flag.NewFlagSet("gx init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	adapter := fs.String("adapter", "", "hypermedia adapter: datastar or htmx; gx init asks when the flag is absent")
	module := fs.String("module", "", "Go module path (default: the directory name)")
	replace := fs.String("replace", "", "use a local checkout of Gx, for work on Gx itself")
	registrySource := fs.String("registry", "", "component registry URL or directory for gx.toml")
	tmpl := fs.String("template", "app", "app, or docs for a docs site with the docs shell")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	if *adapter == "" {
		// One question, with the default in brackets. A closed or empty
		// stdin takes the default, so a script needs no flag.
		fmt.Print("Adapter [datastar]: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Println()
		*adapter = strings.TrimSpace(line)
		if *adapter == "" {
			*adapter = "datastar"
		}
	}
	files, err := scaffold.Init(scaffold.Options{
		Dir: dir, Module: *module, Adapter: *adapter, Version: Version, Replace: *replace, Registry: *registrySource, Template: *tmpl,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx init: %v\n", err)
		return 1
	}
	for _, file := range files {
		fmt.Println("wrote", file)
	}
	fmt.Printf("\nNext:\n  cd %s\n  go run ./cmd/gx dev\n", dir)
	return 0
}

// runNew adds a page, an action, a form, a component or a slice to an app
// (REQ-DEV-10).
func runNew(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: gx new %s <name> [app]\n", strings.Join(scaffold.Kinds, "|"))
		return 2
	}
	dir := "."
	if len(args) > 2 {
		dir = args[2]
	}
	files, err := scaffold.New(dir, args[0], args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx new: %v\n", err)
		return 1
	}
	for _, file := range files {
		fmt.Println("wrote", file)
	}
	return 0
}

// runAgents refreshes the managed section of AGENTS.md (REQ-AI-05).
func runAgents(args []string) int {
	fs := flag.NewFlagSet("gx agents", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	update := fs.Bool("update", false, "rewrite the managed section of AGENTS.md")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !*update {
		fmt.Fprintln(os.Stderr, "usage: gx agents --update [app]")
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	changed, err := scaffold.UpdateAgents(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx agents: %v\n", err)
		return 1
	}
	if changed {
		fmt.Println("updated AGENTS.md")
	} else {
		fmt.Println("AGENTS.md is current")
	}
	return 0
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// The islands are type-checked with the pinned TypeScript compiler
	// (REQ-ISL-08).
	diags, err := tscheck.App(ctx, dir, compiler.CheckOptions{ExternalLinks: *external})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx check: %v\n", err)
		return 1
	}
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
		fmt.Fprintf(os.Stderr, "gx: %v\n", err)
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
		// The pattern already starts with the method.
		line := r.Pattern + "  " + r.Type
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

// runDescribe prints the app model (REQ-AI-01).
func runDescribe(args []string) int {
	fs := flag.NewFlagSet("gx describe", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print machine-readable output")
	schema := fs.Bool("schema", false, "print the JSON Schema of the machine output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *schema {
		_, _ = os.Stdout.Write(compiler.DescribeSchema)
		return 0
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	model, diags, err := appmodel.Describe(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx describe: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		printDiags(diags)
		return 1
	}
	if *asJSON {
		data, err := json.MarshalIndent(model, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "gx describe: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
		return 0
	}
	fmt.Println("module      ", model.Module)
	fmt.Println("components ", len(model.Components))
	for _, c := range model.Components {
		fmt.Printf("  %s.%s  props=%d signals=%d fragments=%d fixtures=%d\n",
			pathBase(c.Package), c.Name, len(c.Props), len(c.Signals), len(c.Fragments), len(c.Fixtures))
	}
	fmt.Println("routes     ", len(model.Routes))
	for _, r := range model.Routes {
		line := "  " + r.Pattern + "  " + r.Kind
		if r.Handler != "" {
			line += "=" + r.Handler
		}
		fmt.Println(line)
	}
	fmt.Println("actions    ", len(model.Actions))
	fmt.Println("forms      ", len(model.Forms))
	fmt.Println("islands    ", len(model.Islands))
	fmt.Println("elements   ", len(model.Elements))
	fmt.Println("transitions", len(model.Transitions))
	for _, t := range model.Transitions {
		fmt.Printf("  %s  %s[%s]\n", t.Name, t.Base, t.Key)
	}
	fmt.Println("icons      ", len(model.Icons))
	for _, i := range model.Icons {
		fmt.Printf("  %s@%s\n", i.Set, i.Version)
	}
	fmt.Println("registry   ", len(model.Registry))
	for _, r := range model.Registry {
		fmt.Printf("  %s@%s\n", r.Name, r.Version)
	}
	return 0
}

// pathBase returns the last element of an import path.
func pathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
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
	if _, err := islands.Write(dir, islands.Options{Minify: true}); err != nil {
		fmt.Fprintf(os.Stderr, "gx build: %v\n", err)
		return 1
	}
	bin := *out
	if bin == "" {
		// The app directory holds app/theme.css, so the binary cannot be
		// <dir>/app: it goes to bin/, named after its main package.
		bin = execname.Name(filepath.Join(dir, "bin", mainName(dir, *main)))
		if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "gx build: %v\n", err)
			return 1
		}
	}
	// go build runs in dir, so a relative output path must not resolve there.
	if abs, err := filepath.Abs(bin); err == nil {
		bin = abs
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

// mainName returns the name of the binary of a main package path.
func mainName(dir, mainPkg string) string {
	name := filepath.Base(filepath.Clean(mainPkg))
	if name == "." || name == string(filepath.Separator) {
		if abs, err := filepath.Abs(dir); err == nil {
			return filepath.Base(abs)
		}
		return "app"
	}
	return name
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
	inst, err := appInstaller(root, source, dir, namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx registry: %v\n", err)
		return registry.Installer{}, 1
	}
	return inst, 0
}

// appInstaller resolves the registry of an app from gx.toml. The commands
// and the dev MCP server share it.
func appInstaller(root, source, dir, namespace string) (registry.Installer, error) {
	cfg, err := gxconfig.Load(root)
	if err != nil {
		return registry.Installer{}, err
	}
	src := source
	var headers []string
	if src == "" && namespace != "" {
		named, ok := cfg.Registries[namespace]
		if !ok {
			return registry.Installer{}, fmt.Errorf("unknown registry @%s", namespace)
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
		return registry.Installer{}, errors.New("no registry; set [registry] url in gx.toml or pass --registry")
	}
	installDir := dir
	if installDir == "" {
		installDir = cfg.Registry.Dir
	}
	return registry.Installer{Root: root, Source: src, Dir: installDir, Headers: headers}, nil
}

// runMCP runs the dev MCP server on stdio (REQ-AI-04). Stdout carries the
// protocol, so the command prints nothing else there.
func runMCP(args []string) int {
	fs := flag.NewFlagSet("gx mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	mainPkg := fs.String("main", "", "app main package")
	source := fs.String("registry", "", "registry URL or directory (overrides [registry] url)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	err := mcpserver.Run(ctx, mcpserver.Options{
		Dir:     dir,
		Main:    *mainPkg,
		Version: Version,
		Installer: func() (registry.Installer, error) {
			return appInstaller(dir, *source, "", "")
		},
	}, mcpserver.Stdio())
	if err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "gx mcp: %v\n", err)
		return 1
	}
	return 0
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
// runRegistry builds or lints a publishable registry (REQ-REG-01,
// REQ-REG-06).
func runRegistry(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx registry build|lint <src>")
		return 2
	}
	switch args[0] {
	case "build":
		return runRegistryBuild(args[1:])
	case "lint":
		return runRegistryLint(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: gx registry build|lint <src>")
		return 2
	}
}

// runRegistryBuild writes the published registry of a source folder
// (REQ-REG-01).
func runRegistryBuild(args []string) int {
	fs := flag.NewFlagSet("gx registry build", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "", "output directory (default the source directory)")
	if err := fs.Parse(args); err != nil {
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

// propDocs reads the props of one .gx source for the registry lint.
func propDocs(path string, src []byte) ([]registry.PropDoc, error) {
	file, diags := compiler.ParseFile(path, src)
	if len(diags) > 0 {
		return nil, errors.New(diags[0].Msg)
	}
	out := make([]registry.PropDoc, 0, len(file.Props))
	for _, prop := range file.Props {
		out = append(out, registry.PropDoc{Name: prop.Name, Doc: prop.Doc})
	}
	return out, nil
}

// runRegistryLint fails an item that lacks fixtures, usage sections, a
// keyboard spec or a prop description (REQ-REG-06).
func runRegistryLint(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx registry lint <src>")
		return 2
	}
	findings, err := registry.Lint(args[0], propDocs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx registry lint: %v\n", err)
		return 1
	}
	for _, finding := range findings {
		fmt.Println(finding)
	}
	if len(findings) > 0 {
		fmt.Fprintf(os.Stderr, "gx registry lint: %d findings\n", len(findings))
		return 1
	}
	fmt.Println("registry lint: ok")
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
	tsPath, err := (&tscheck.Manager{Root: dir}).Vendor(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx vendor: typescript: %v\n", err)
		return 1
	}
	fmt.Println("vendored", tsPath)
	if !tscheck.HasLock(dir) {
		if err := tscheck.SaveLock(dir, tscheck.DefaultLock()); err != nil {
			fmt.Fprintf(os.Stderr, "gx vendor: typescript lock: %v\n", err)
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

// runPin vendors the pre-bundled ESM build of an npm package (REQ-ISL-07).
func runPin(args []string) int {
	fs := flag.NewFlagSet("gx pin", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cdn := fs.String("cdn", "", "ESM CDN (default the esm mirror of gx.toml, then jsDelivr)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gx pin [--cdn <url>] <pkg>@<version> [app]")
		return 2
	}
	dir := "."
	if len(rest) > 1 {
		dir = rest[1]
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	res, err := jspin.Pin(ctx, jspin.Options{Dir: dir, Spec: rest[0], BaseURL: *cdn})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("pinned %s@%s: %d files in %s\n", res.Specifier, res.Version, len(res.Files), jspin.VendorDir)
	if len(res.Removed) > 0 {
		fmt.Printf("removed %d files that no pin uses\n", len(res.Removed))
	}
	return 0
}

// stringList is a flag that the command line can give more than once.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

// runWC imports the web components of an npm package (REQ-ISL-09).
func runWC(args []string) int {
	const usageLine = "usage: gx wc pin [--as <name>] [--out <dir>] [--element <tag>]... [--cdn <url>] <pkg>@<version> [app]"
	if len(args) > 0 && args[0] == "build" {
		return runWCBuild(args[1:])
	}
	if len(args) == 0 || args[0] != "pin" {
		fmt.Fprintln(os.Stderr, usageLine)
		fmt.Fprintln(os.Stderr, wcBuildUsage)
		return 2
	}
	fs := flag.NewFlagSet("gx wc pin", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	as := fs.String("as", "", "Go package name of the tags (default the prefix of the tag names)")
	out := fs.String("out", "", "parent directory of the package (default ui)")
	cdn := fs.String("cdn", "", "ESM CDN (default the esm mirror of gx.toml, then jsDelivr)")
	var elements stringList
	fs.Var(&elements, "element", "import this tag only; give the flag again for more tags")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, usageLine)
		return 2
	}
	dir := "."
	if len(rest) > 1 {
		dir = rest[1]
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	res, err := wcpin.Pin(ctx, wcpin.Options{Dir: dir, Spec: rest[0], As: *as, Out: *out, Elements: elements, BaseURL: *cdn})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("imported %d elements of %s into %s: %s\n", len(res.Tags), rest[0], res.Package, strings.Join(res.Tags, ", "))
	return 0
}

const wcBuildUsage = "usage: gx wc build [--server <origin>] [--base <path>] [--out <dir>] [app or package]"

// runWCBuild writes the files of each widget for a host page (REQ-ISL-10):
// the element file, a type file, the JSX types for React, and one custom
// elements manifest. The directory is the app, or one package of it.
func runWCBuild(args []string) int {
	fs := flag.NewFlagSet("gx wc build", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	server := fs.String("server", "", "origin of the Gx server, for example https://api.acme.dev (default the origin of the host page)")
	base := fs.String("base", "", "base path of the app on the server, as Config.BasePath")
	out := fs.String("out", "", "directory of the files (default dist/widgets in the app)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 1 {
		fmt.Fprintln(os.Stderr, wcBuildUsage)
		return 2
	} else if len(rest) == 1 {
		dir = rest[0]
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx wc build: %v\n", err)
		return 1
	}
	root := moduleRootOf(dir)
	if root == "" {
		fmt.Fprintf(os.Stderr, "gx wc build: %s is not in a Go module\n", dir)
		return 1
	}
	if diags := compiler.CheckApp(root, compiler.CheckOptions{}); len(diags) > 0 {
		printDiags(diags)
		return 1
	}
	all, diags := compiler.Widgets(root)
	if len(diags) > 0 {
		printDiags(diags)
		return 1
	}
	var widgets []compiler.WidgetBuild
	for _, w := range all {
		if rel, err := filepath.Rel(dir, w.Dir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			widgets = append(widgets, w)
		}
	}
	if len(widgets) == 0 {
		fmt.Fprintf(os.Stderr, "gx wc build: %s has no widget. Declare one with gx.Widget(load, view).Tag(\"acme-name\").\n", dir)
		return 1
	}
	if *out == "" {
		*out = filepath.Join(root, "dist", "widgets")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gx wc build: %v\n", err)
		return 1
	}
	basePath := strings.TrimSuffix(*base, "/")
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	write := func(name string, data []byte) bool {
		if err := os.WriteFile(filepath.Join(*out, name), data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "gx wc build: %v\n", err)
			return false
		}
		return true
	}
	for _, w := range widgets {
		attrs := make([]string, len(w.Attributes))
		for i, a := range w.Attributes {
			attrs[i] = a.Name
		}
		element, err := widgetelement.File(widgetelement.Config{Tag: w.Tag, Attrs: attrs, Server: *server, Path: basePath + w.Path})
		if err != nil {
			fmt.Fprintf(os.Stderr, "gx wc build: %v\n", err)
			return 1
		}
		if !write(w.Tag+".js", element) || !write(w.Tag+".d.ts", w.DTS) || !write(w.Tag+".react.d.ts", w.ReactDTS) {
			return 1
		}
	}
	manifest, err := compiler.WidgetManifest(widgets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx wc build: %v\n", err)
		return 1
	}
	if !write("custom-elements.json", manifest) {
		return 1
	}
	tags := make([]string, len(widgets))
	for i, w := range widgets {
		tags[i] = w.Tag
	}
	fmt.Printf("wrote %d widgets to %s: %s\n", len(widgets), *out, strings.Join(tags, ", "))
	if *server == "" {
		fmt.Println("no --server: each element calls the origin of its host page")
	}
	return 0
}

// moduleRootOf returns the directory of the go.mod at or above dir, or "".
func moduleRootOf(dir string) string {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

func runLint(args []string) int {
	fs := flag.NewFlagSet("gx lint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print machine-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if rest := fs.Args(); len(rest) > 0 {
		dir = rest[0]
	}
	// go vet has no Gx codes; its text stays off stdout so --json is clean.
	vetErr := runVet(dir, os.Stderr)
	findings, err := analyze.Lint(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx lint: %v\n", err)
		return 1
	}
	if *asJSON {
		diags := make([]compiler.Diagnostic, 0, len(findings))
		for _, f := range findings {
			diags = append(diags, compiler.Diagnostic{Code: f.Code, File: f.File, Line: f.Line, Col: f.Col, Msg: f.Message})
		}
		if code := printJSON(diags); code != 0 || vetErr == nil {
			return code
		}
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

// routeTagLine matches the embedded route field of a route type: the tag
// holds a method and a pattern, which is not the key:"value" form go vet
// expects (REQ-RTE-01).
var routeTagLine = regexp.MustCompile("^\\s*\\w+\\.Route\\s+`")

// runVet runs go vet on every package of dir and prints its findings to w.
// It leaves out one finding: the struct tag of a gx.Route field. It returns
// an error when a finding stays or a package does not compile (REQ-TLS-03).
func runVet(dir string, w io.Writer) error {
	cmd := exec.Command("go", "vet", "-json", "./...")
	cmd.Dir = dir
	var raw bytes.Buffer
	cmd.Stdout, cmd.Stderr = &raw, &raw
	runErr := cmd.Run()

	type finding struct {
		Posn    string `json:"posn"`
		Message string `json:"message"`
	}
	abs, _ := filepath.Abs(dir)
	found := 0
	report := func(analyzer string, f finding) {
		if analyzer == "structtag" && isRouteTag(f.Posn) {
			return
		}
		found++
		posn := f.Posn
		if rel, err := filepath.Rel(abs, posn); err == nil && !strings.HasPrefix(rel, "..") {
			posn = rel
		}
		fmt.Fprintf(w, "%s: %s\n", posn, f.Message)
	}
	// The output mixes "# package" lines, compiler errors and one JSON
	// object per package with findings.
	var object []string
	for _, line := range strings.Split(raw.String(), "\n") {
		if len(object) == 0 && line != "{" {
			// A package with no finding prints an empty object.
			if text := strings.TrimSpace(line); text != "" && text != "{}" && !strings.HasPrefix(line, "# ") {
				fmt.Fprintln(w, line)
			}
			continue
		}
		object = append(object, line)
		if line != "}" {
			continue
		}
		var packages map[string]map[string]json.RawMessage
		if err := json.Unmarshal([]byte(strings.Join(object, "\n")), &packages); err != nil {
			fmt.Fprintln(w, strings.Join(object, "\n"))
			found++
		}
		object = nil
		for _, analyzers := range packages {
			names := make([]string, 0, len(analyzers))
			for name := range analyzers {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				var findings []finding
				if err := json.Unmarshal(analyzers[name], &findings); err != nil {
					// An analyzer that failed gives an object with
					// its error.
					fmt.Fprintf(w, "%s: %s\n", name, analyzers[name])
					found++
					continue
				}
				for _, f := range findings {
					report(name, f)
				}
			}
		}
	}
	if runErr != nil {
		return runErr
	}
	if found > 0 {
		return fmt.Errorf("go vet: %d findings", found)
	}
	return nil
}

// isRouteTag reports whether a vet position is the embedded gx.Route field
// of a route type.
func isRouteTag(posn string) bool {
	// The position is file:line:column; a Windows path has a drive colon.
	parts := strings.Split(posn, ":")
	if len(parts) < 3 {
		return false
	}
	line, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return false
	}
	data, err := os.ReadFile(strings.Join(parts[:len(parts)-2], ":"))
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	return line >= 1 && line <= len(lines) && routeTagLine.MatchString(lines[line-1])
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
	// A closed terminal sends SIGHUP; without it the app outlives gx dev.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
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
