// Package islands bundles the TypeScript islands of an app with esbuild and
// writes the generated package that embeds the bundle (REQ-ISL-03). The
// app's main calls gx.SetIslands with gxislands.Bundle(). esbuild is a Go
// library inside the gx command, so no node is involved.
package islands

import (
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/jspin"
)

// Dir and File name the generated package.
const (
	Dir  = "gxislands"
	File = "islands_gx.go"
)

// Path returns the generated file path under root.
func Path(root string) string { return filepath.Join(root, Dir, File) }

// Bundle is the built JavaScript of the islands of one app.
type Bundle struct {
	// Entries maps the name of an island (the import path of its package
	// and its component name) to its file.
	Entries map[string]string
	// Files maps a file name to its content. A name holds the hash of the
	// content.
	Files map[string][]byte
}

// Message is one esbuild error.
type Message struct {
	File string
	Line int
	Col  int
	Text string
}

// Error holds the errors of a failed bundle.
type Error struct {
	Messages []Message
}

func (e *Error) Error() string {
	lines := make([]string, len(e.Messages))
	for i, m := range e.Messages {
		if m.File == "" {
			lines[i] = m.Text
			continue
		}
		lines[i] = fmt.Sprintf("%s:%d:%d: %s", m.File, m.Line, m.Col, m.Text)
	}
	return strings.Join(lines, "\n")
}

// Options configure one bundle.
type Options struct {
	// Minify makes the production form. The dev form keeps names and has
	// source maps.
	Minify bool
}

// Build bundles every island under root. Each island is one ESM entry
// file, and code that two islands share goes to a common chunk.
func Build(root string, opt Options) (*Bundle, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	refs := compiler.Islands(root)
	// The islands of the directories that [islands] roots of gx.toml names:
	// packages that the app imports and that are not under the app.
	cfg, err := gxconfig.Load(root)
	if err != nil {
		return nil, err
	}
	for _, extra := range cfg.IslandRoots {
		dir := extra
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, filepath.FromSlash(extra))
		}
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("islands: the root %q of gx.toml: %w", extra, err)
		}
		refs = append(refs, compiler.Islands(dir)...)
	}
	modules := compiler.ElementModules(root)
	out := &Bundle{Entries: map[string]string{}, Files: map[string][]byte{}}
	if len(refs) == 0 && len(modules) == 0 {
		return out, nil
	}
	lock, err := jspin.LoadLock(root)
	if err != nil {
		return nil, err
	}
	nodeModules := jspin.UsesNodeModules(root)
	entries := make([]api.EntryPoint, 0, len(refs)+len(modules))
	// byOutput maps the output name of an entry, with no hash and no
	// extension, to its name in the bundle.
	byOutput := map[string]string{}
	for _, ref := range refs {
		entries = append(entries, api.EntryPoint{InputPath: ref.File, OutputPath: ref.ID})
		byOutput[ref.ID] = ref.ID
	}
	// The module of each imported web component in use is an entry too
	// (REQ-ISL-09). Its name in the bundle is its import specifier.
	for _, module := range modules {
		input := module
		if behavior, ok := compiler.DirectiveBehavior(module); ok {
			// The behaviour of a plugin directive is a file of the
			// plugin, not of a package (REQ-PLG-03).
			input = filepath.Join(root, ".gx", "directives", filepath.FromSlash(module)+".js")
			if err := os.MkdirAll(filepath.Dir(input), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(input, behavior, 0o644); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(module, "gx-directive/") {
			return nil, fmt.Errorf("islands: a .gx file uses the directive of the module %q, and no plugin of this command has it; run the cmd/gx of the project", module)
		} else if !nodeModules {
			pin, ok := lock.Pins[module]
			if !ok {
				return nil, fmt.Errorf("islands: the web component module %q has no pin; run gx wc pin again", module)
			}
			input = filepath.Join(root, filepath.FromSlash(pin.File))
		}
		output := "elements/" + elementOutput(module)
		entries = append(entries, api.EntryPoint{InputPath: input, OutputPath: output})
		byOutput[output] = module
	}
	outdir := filepath.Join(root, ".gx", "islands")
	options := api.BuildOptions{
		AbsWorkingDir:       root,
		EntryPointsAdvanced: entries,
		Outdir:              outdir,
		Write:               false,
		Bundle:              true,
		Splitting:           true,
		Format:              api.FormatESModule,
		Platform:            api.PlatformBrowser,
		Target:              api.ES2022,
		EntryNames:          "[dir]/[name]-[hash]",
		ChunkNames:          "chunks/[name]-[hash]",
		AssetNames:          "assets/[name]-[hash]",
		Metafile:            true,
		LogLevel:            api.LogLevelSilent,
		Charset:             api.CharsetUTF8,
	}
	if opt.Minify {
		options.MinifyWhitespace = true
		options.MinifyIdentifiers = true
		options.MinifySyntax = true
	} else {
		options.Sourcemap = api.SourceMapLinked
	}
	// A bare import is a pinned package of gx.lock (REQ-ISL-07). An app
	// with package.json and node_modules resolves as node does.
	// A vendored file that differs from gx.lock stops the build (SI-10).
	if err := jspin.Verify(root, lock); err != nil {
		return nil, err
	}
	if !nodeModules {
		options.Plugins = []api.Plugin{pinPlugin(root, lock)}
	}
	res := api.Build(options)
	if len(res.Errors) > 0 {
		e := &Error{}
		for _, m := range res.Errors {
			msg := Message{Text: m.Text}
			if m.Location != nil {
				msg.File = filepath.Join(root, filepath.FromSlash(m.Location.File))
				msg.Line = m.Location.Line
				msg.Col = m.Location.Column + 1
			}
			e.Messages = append(e.Messages, msg)
		}
		return nil, e
	}
	for _, f := range res.OutputFiles {
		rel, err := filepath.Rel(outdir, f.Path)
		if err != nil {
			return nil, err
		}
		out.Files[filepath.ToSlash(rel)] = f.Contents
	}
	var meta struct {
		Outputs map[string]struct {
			EntryPoint string `json:"entryPoint"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(res.Metafile), &meta); err != nil {
		return nil, fmt.Errorf("islands: metafile: %w", err)
	}
	outRel, err := filepath.Rel(root, outdir)
	if err != nil {
		return nil, err
	}
	prefix := filepath.ToSlash(outRel) + "/"
	for path, o := range meta.Outputs {
		if o.EntryPoint == "" {
			continue
		}
		// An entry file is <output name>-<hash>.js.
		file := strings.TrimPrefix(path, prefix)
		if m := entryFile.FindStringSubmatch(file); m != nil {
			if id, ok := byOutput[m[1]]; ok {
				out.Entries[id] = file
			}
		}
	}
	for _, id := range byOutput {
		if _, ok := out.Entries[id]; !ok {
			return nil, fmt.Errorf("islands: the bundle has no entry for %s", id)
		}
	}
	return out, nil
}

// entryFile splits the name of an entry file into its output name and its
// content hash.
var entryFile = regexp.MustCompile(`^(.+)-[0-9A-Z]{8}\.js$`)

// elementOutput makes a file name from the import specifier of a module.
func elementOutput(module string) string {
	name := strings.TrimSuffix(strings.TrimSuffix(module, ".js"), ".mjs")
	return strings.Trim(notFileChar.ReplaceAllString(name, "-"), "-")
}

var notFileChar = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// pinPlugin resolves a bare import to the vendored file of its pin.
func pinPlugin(root string, lock jspin.Lock) api.Plugin {
	return api.Plugin{Name: "gx-pins", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: `^[^./]`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			if args.Kind == api.ResolveEntryPoint {
				return api.OnResolveResult{}, nil
			}
			if pin, ok := lock.Pins[args.Path]; ok {
				return api.OnResolveResult{Path: filepath.Join(root, filepath.FromSlash(pin.File))}, nil
			}
			return api.OnResolveResult{Errors: []api.Message{{
				Text: fmt.Sprintf("the import %q has no pin; run: gx pin %s@<version>", args.Path, args.Path),
			}}}, nil
		})
	}}
}

// Generate returns the Go source that embeds a bundle.
func Generate(b *Bundle) []byte {
	var s strings.Builder
	s.WriteString("// Code generated by gx. DO NOT EDIT.\n\n")
	s.WriteString("package gxislands\n\n")
	s.WriteString("import gx \"github.com/alternayte/gx\"\n\n")
	s.WriteString("// Bundle returns the built islands of the app (REQ-ISL-03).\n")
	s.WriteString("func Bundle() gx.IslandBundle {\n")
	s.WriteString("\treturn gx.IslandBundle{Entries: entries, Files: files}\n}\n\n")
	writeMap := func(name string, keys []string, value func(string) string) {
		if len(keys) == 0 {
			s.WriteString("var " + name + " = map[string]string{}\n")
			return
		}
		s.WriteString("var " + name + " = map[string]string{\n")
		for _, k := range keys {
			s.WriteString("\t" + strconv.Quote(k) + ": " + strconv.Quote(value(k)) + ",\n")
		}
		s.WriteString("}\n")
	}
	var entries, files []string
	if b != nil {
		for k := range b.Entries {
			entries = append(entries, k)
		}
		for k := range b.Files {
			files = append(files, k)
		}
	}
	sort.Strings(entries)
	sort.Strings(files)
	writeMap("entries", entries, func(k string) string { return b.Entries[k] })
	s.WriteString("\n")
	writeMap("files", files, func(k string) string { return string(b.Files[k]) })
	if formatted, err := format.Source([]byte(s.String())); err == nil {
		return formatted
	}
	return []byte(s.String())
}

// Write builds the islands of the app and writes the generated package. An
// app with no island and no generated package gets no file. It reports
// whether it wrote the file.
func Write(root string, opt Options) (bool, error) {
	b, err := Build(root, opt)
	if err != nil {
		return false, err
	}
	path := Path(root)
	if len(b.Entries) == 0 {
		if _, err := os.Stat(path); err != nil {
			return false, nil
		}
	}
	src := Generate(b)
	if old, err := os.ReadFile(path); err == nil && string(old) == string(src) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, src, 0o644)
}
