package gxcli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/gxstyles"
	"github.com/alternayte/gx/internal/widgetelement"
	"github.com/alternayte/gx/internal/widgetpkg"
)

const (
	wcBuildUsage   = "usage: gx wc build [--server <origin>] [--base <path>] [--out <dir>] [app or package]"
	wcCheckUsage   = "usage: gx wc check [--update] [app]"
	wcPackUsage    = "usage: gx wc pack [--server <origin>] [--base <path>] [--out <dir>] [app]"
	wcPublishUsage = "usage: gx wc publish [--server <origin>] [--base <path>] [--registry <url>] [--tag <name>] [app]"
)

// widgetSet is the widgets of an app, with the files that a host gets.
type widgetSet struct {
	// root is the app: the directory of go.mod and gx.toml.
	root    string
	cfg     gxconfig.Widgets
	server  string
	widgets []compiler.WidgetBuild
	// files are the element file and the type files of each widget, and
	// the manifest.
	files    []widgetpkg.File
	manifest []byte
}

// loadWidgetSet checks the app and makes the files of its widgets. dir is
// the app, or one package of it: the set then holds the widgets of that
// package only. The flags server and base go before the [widgets] table of
// gx.toml. It prints each problem and returns nil for one.
func loadWidgetSet(ctx context.Context, cmd, dir, server, base string) *widgetSet {
	fail := func(format string, args ...any) *widgetSet {
		fmt.Fprintf(os.Stderr, cmd+": "+format+"\n", args...)
		return nil
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fail("%v", err)
	}
	root := moduleRootOf(dir)
	if root == "" {
		return fail("%s is not in a Go module", dir)
	}
	cfg, err := gxconfig.Load(root)
	if err != nil {
		return fail("%v", err)
	}
	if diags := compiler.CheckApp(root, compiler.CheckOptions{}); len(diags) > 0 {
		printDiags(diags)
		return nil
	}
	all, diags := compiler.Widgets(root)
	if len(diags) > 0 {
		printDiags(diags)
		return nil
	}
	set := &widgetSet{root: root, cfg: cfg.Widgets, server: server}
	for _, w := range all {
		if rel, err := filepath.Rel(dir, w.Dir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			set.widgets = append(set.widgets, w)
		}
	}
	if len(set.widgets) == 0 {
		return fail("%s has no widget. Declare one with gx.Widget(load, view).Tag(\"acme-name\").", dir)
	}
	if set.server == "" {
		set.server = cfg.Widgets.Server
	}
	if base == "" {
		base = cfg.Widgets.Base
	}
	base = strings.TrimSuffix(base, "/")
	if base != "" && !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	// The CSS variables of a widget come from its stylesheet, so the
	// stylesheet is built as gx build makes it. An app with no theme has
	// no stylesheet of a widget.
	if _, err := os.Stat(gxstyles.ThemePath(root)); err == nil {
		lists := map[string][]string{}
		for _, w := range set.widgets {
			lists[w.Tag] = w.Classes
		}
		sheets, err := gxstyles.BuildWidgetLists(ctx, root, lists)
		if err != nil {
			return fail("%v", err)
		}
		for i := range set.widgets {
			set.widgets[i].CSSVariables = gxstyles.WidgetTokens(sheets[set.widgets[i].Tag])
		}
	}
	for _, w := range set.widgets {
		attrs := make([]string, len(w.Attributes))
		for i, a := range w.Attributes {
			attrs[i] = a.Name
		}
		element, err := widgetelement.File(widgetelement.Config{Tag: w.Tag, Attrs: attrs, Server: set.server, Path: base + w.Path})
		if err != nil {
			return fail("%v", err)
		}
		set.files = append(set.files,
			widgetpkg.File{Name: w.Tag + ".js", Data: element},
			widgetpkg.File{Name: w.Tag + ".d.ts", Data: w.DTS},
			widgetpkg.File{Name: w.Tag + ".react.d.ts", Data: w.ReactDTS})
	}
	if set.manifest, err = compiler.WidgetManifest(set.widgets); err != nil {
		return fail("%v", err)
	}
	set.files = append(set.files, widgetpkg.File{Name: widgetpkg.ManifestFile, Data: set.manifest})
	return set
}

func (s *widgetSet) tags() []string {
	tags := make([]string, len(s.widgets))
	for i, w := range s.widgets {
		tags[i] = w.Tag
	}
	return tags
}

// pkg returns the npm package of the set. It prints the problem and returns
// false when gx.toml does not name the package.
func (s *widgetSet) pkg(cmd string) (widgetpkg.Package, bool) {
	p := widgetpkg.Package{Name: s.cfg.Name, Version: s.cfg.Version, Tags: s.tags(), Files: s.files}
	if p.Name == "" || p.Version == "" {
		fmt.Fprintf(os.Stderr, "%s: gx.toml does not name the package of the widgets. Add:\n\n[widgets]\nname = \"@acme/widgets\"\nversion = \"0.1.0\"\n", cmd)
		return p, false
	}
	if _, err := p.PackageJSON(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: gx.toml, [widgets]: %v\n", cmd, err)
		return p, false
	}
	return p, true
}

// noServerNote tells the user what an element file with no server does.
func (s *widgetSet) noServerNote() {
	if s.server == "" {
		fmt.Println("no --server and no server in [widgets] of gx.toml: each element calls the origin of its host page")
	}
}

// wcFlags are the flags that each command with element files has.
func wcFlags(fs *flag.FlagSet) (server, base *string) {
	server = fs.String("server", "", "origin of the Gx server, for example https://api.acme.dev (default the server of [widgets] in gx.toml, then the origin of the host page)")
	base = fs.String("base", "", "base path of the app on the server, as Config.BasePath (default the base of [widgets] in gx.toml)")
	return server, base
}

// wcDir returns the one optional directory argument of a command.
func wcDir(fs *flag.FlagSet, usageLine string) (string, bool) {
	switch rest := fs.Args(); len(rest) {
	case 0:
		return ".", true
	case 1:
		return rest[0], true
	}
	fmt.Fprintln(os.Stderr, usageLine)
	return "", false
}

// runWCBuild writes the files of each widget for a host page (REQ-ISL-10):
// the element file, a type file, the JSX types for React, and one custom
// elements manifest. The directory is the app, or one package of it.
func runWCBuild(args []string) int {
	fs := flag.NewFlagSet("gx wc build", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	server, base := wcFlags(fs)
	out := fs.String("out", "", "directory of the files (default dist/widgets in the app)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir, ok := wcDir(fs, wcBuildUsage)
	if !ok {
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	set := loadWidgetSet(ctx, "gx wc build", dir, *server, *base)
	if set == nil {
		return 1
	}
	if *out == "" {
		*out = filepath.Join(set.root, "dist", "widgets")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gx wc build: %v\n", err)
		return 1
	}
	for _, f := range set.files {
		if err := os.WriteFile(filepath.Join(*out, f.Name), f.Data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "gx wc build: %v\n", err)
			return 1
		}
	}
	fmt.Printf("wrote %d widgets to %s: %s\n", len(set.widgets), *out, strings.Join(set.tags(), ", "))
	set.noServerNote()
	return 0
}

// checkContract compares the manifest of the set with the baseline of the
// app and prints each change (REQ-ISL-13). It returns false when the
// version of gx.toml does not have the bump that the changes need.
func (s *widgetSet) checkContract(cmd string) bool {
	now, err := widgetpkg.ParseVersion(s.cfg.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: gx.toml, [widgets]: %v\n", cmd, err)
		return false
	}
	baseline, err := widgetpkg.ReadBaseline(s.root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd, err)
		return false
	}
	if baseline == nil {
		fmt.Printf("no baseline at %s: no version of the widgets is published, so each version passes\n", widgetpkg.BaselinePath(s.root))
		return true
	}
	old, err := widgetpkg.ParseVersion(baseline.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", cmd, widgetpkg.BaselinePath(s.root), err)
		return false
	}
	changes, err := widgetpkg.Compare(baseline.Manifest, s.manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd, err)
		return false
	}
	for _, c := range changes {
		fmt.Printf("%s: %s\n", c.Level, c.Text)
	}
	level := widgetpkg.Needed(changes)
	if err := widgetpkg.CheckBump(old, now, level); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v. Change the version of [widgets] in gx.toml.\n", cmd, err)
		return false
	}
	if level == widgetpkg.None {
		fmt.Printf("the contract of the widgets is the contract of the baseline %s\n", old)
	} else {
		fmt.Printf("the version %s has the %s bump that the changes need; the baseline is %s\n", now, level, old)
	}
	return true
}

// writeBaseline records the manifest of the set as the baseline of the app.
func (s *widgetSet) writeBaseline(cmd string) bool {
	if err := widgetpkg.WriteBaseline(s.root, widgetpkg.Baseline{Name: s.cfg.Name, Version: s.cfg.Version, Manifest: s.manifest}); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd, err)
		return false
	}
	fmt.Printf("wrote the baseline %s to %s; commit the file\n", s.cfg.Version, widgetpkg.BaselinePath(s.root))
	return true
}

// runWCCheck compares the contract of the widgets with the last published
// baseline (REQ-ISL-13). A breaking change with no major bump, or an
// addition with no minor bump, is exit code 1.
func runWCCheck(args []string) int {
	fs := flag.NewFlagSet("gx wc check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	update := fs.Bool("update", false, "after the check passes, record the contract of now as the baseline; for an app that publishes no package")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir, ok := wcDir(fs, wcCheckUsage)
	if !ok {
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	set := loadWidgetSet(ctx, "gx wc check", dir, "", "")
	if set == nil {
		return 1
	}
	if set.root != mustAbs(dir) {
		fmt.Fprintln(os.Stderr, "gx wc check: the directory is one package. The contract is for each widget of the app: give the app.")
		return 2
	}
	if set.cfg.Version == "" {
		fmt.Fprint(os.Stderr, "gx wc check: gx.toml has no version of the widgets. Add:\n\n[widgets]\nversion = \"0.1.0\"\n")
		return 1
	}
	if !set.checkContract("gx wc check") {
		return 1
	}
	if *update && !set.writeBaseline("gx wc check") {
		return 1
	}
	return 0
}

func mustAbs(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// runWCPack writes the npm tarball of the widgets of the app (REQ-ISL-14).
func runWCPack(args []string) int {
	fs := flag.NewFlagSet("gx wc pack", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	server, base := wcFlags(fs)
	out := fs.String("out", "", "directory of the tarball (default dist/widgets in the app)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir, ok := wcDir(fs, wcPackUsage)
	if !ok {
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	set := loadWidgetSet(ctx, "gx wc pack", dir, *server, *base)
	if set == nil {
		return 1
	}
	if set.root != mustAbs(dir) {
		fmt.Fprintln(os.Stderr, "gx wc pack: the directory is one package. The tarball holds each widget of the app: give the app.")
		return 2
	}
	p, ok := set.pkg("gx wc pack")
	if !ok {
		return 1
	}
	tarball, err := p.Tarball()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx wc pack: %v\n", err)
		return 1
	}
	if *out == "" {
		*out = filepath.Join(set.root, "dist", "widgets")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gx wc pack: %v\n", err)
		return 1
	}
	path := filepath.Join(*out, widgetpkg.TarballName(p.Name, p.Version))
	if err := os.WriteFile(path, tarball, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gx wc pack: %v\n", err)
		return 1
	}
	fmt.Printf("wrote %s: %s %s with %s\n", path, p.Name, p.Version, strings.Join(p.Tags, ", "))
	set.noServerNote()
	return 0
}

// npmTokenEnv is the environment variable with the token of the npm
// registry. The token is never a flag: a flag stays in the shell history.
const npmTokenEnv = "NPM_TOKEN"

// runWCPublish publishes the npm package of the widgets through the HTTP
// API of the registry, with no node (REQ-ISL-14). It runs the contract
// check first, and records the baseline after the registry takes the
// version.
func runWCPublish(args []string) int {
	fs := flag.NewFlagSet("gx wc publish", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	server, base := wcFlags(fs)
	registry := fs.String("registry", "", "URL of the npm registry (default the registry of [widgets] in gx.toml, then "+widgetpkg.DefaultRegistry+")")
	tag := fs.String("tag", "latest", "dist-tag of the version")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir, ok := wcDir(fs, wcPublishUsage)
	if !ok {
		return 2
	}
	token := os.Getenv(npmTokenEnv)
	if token == "" {
		fmt.Fprintln(os.Stderr, "gx wc publish: the environment has no "+npmTokenEnv+". Set it to a token of the registry that can publish the package.")
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	set := loadWidgetSet(ctx, "gx wc publish", dir, *server, *base)
	if set == nil {
		return 1
	}
	if set.root != mustAbs(dir) {
		fmt.Fprintln(os.Stderr, "gx wc publish: the directory is one package. The package holds each widget of the app: give the app.")
		return 2
	}
	p, ok := set.pkg("gx wc publish")
	if !ok {
		return 1
	}
	if !set.checkContract("gx wc publish") {
		return 1
	}
	if *registry == "" {
		*registry = set.cfg.Registry
	}
	if *registry == "" {
		*registry = widgetpkg.DefaultRegistry
	}
	err := widgetpkg.Publish(ctx, p, widgetpkg.PublishOptions{Registry: *registry, Token: token, Tag: *tag, Access: set.cfg.Access})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gx wc publish: %v\n", err)
		return 1
	}
	fmt.Printf("published %s %s to %s with %s\n", p.Name, p.Version, *registry, strings.Join(p.Tags, ", "))
	set.noServerNote()
	if !set.writeBaseline("gx wc publish") {
		return 1
	}
	return 0
}
