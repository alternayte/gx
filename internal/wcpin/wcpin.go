// Package wcpin imports web components from npm (REQ-ISL-09). `gx wc pin
// <pkg>@<version>` reads the custom elements manifest of the package, pins
// the module of each element with gx pin, and writes a Go package that
// holds one typed tag for each element.
package wcpin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/jspin"
)

// manifest is the part of a custom elements manifest (schema 1.x) that the
// import reads.
type manifest struct {
	Modules []struct {
		Path         string `json:"path"`
		Declarations []struct {
			Kind          string `json:"kind"`
			Name          string `json:"name"`
			TagName       string `json:"tagName"`
			CustomElement bool   `json:"customElement"`
			Description   string `json:"description"`
			Summary       string `json:"summary"`
			Attributes    []struct {
				Name        string    `json:"name"`
				Type        *typeText `json:"type"`
				Description string    `json:"description"`
				FieldName   string    `json:"fieldName"`
			} `json:"attributes"`
			Events []struct {
				Name string `json:"name"`
			} `json:"events"`
			Slots []struct {
				Name string `json:"name"`
			} `json:"slots"`
		} `json:"declarations"`
	} `json:"modules"`
}

type typeText struct {
	Text string `json:"text"`
}

// Options configure one import.
type Options struct {
	// Dir is the app directory.
	Dir string
	// Spec is <pkg>@<version>.
	Spec string
	// As is the Go package name of the tags. The default is the prefix
	// that the tag names share ("wa" for wa-button), or the last part of
	// the package name.
	As string
	// Out is the parent directory of the package, relative to Dir. The
	// default is "ui".
	Out string
	// Elements limits the import to these tag names. The default is every
	// element of the manifest.
	Elements []string
	// BaseURL overrides the CDN, as for gx pin.
	BaseURL string
	// Client is the HTTP client.
	Client *http.Client
}

// Result reports one import.
type Result struct {
	// Package is the directory of the written package, relative to Dir.
	Package string
	// Tags lists the imported tag names.
	Tags []string
}

var specPattern = regexp.MustCompile(`^((?:@[a-z0-9][a-z0-9._~-]*/)?[a-z0-9][a-z0-9._~-]*)@([A-Za-z0-9][A-Za-z0-9.+_-]*)$`)

// Pin imports the custom elements of a package.
func Pin(ctx context.Context, opt Options) (*Result, error) {
	m := specPattern.FindStringSubmatch(opt.Spec)
	if m == nil {
		return nil, fmt.Errorf("gx wc pin: %q is not <pkg>@<version>, for example @awesome.me/webawesome@3.14.0", opt.Spec)
	}
	pkg, version := m[1], m[2]
	base := opt.BaseURL
	if base == "" {
		if cfg, err := gxconfig.Load(opt.Dir); err == nil {
			base = cfg.Mirror("esm")
		}
	}
	if base == "" {
		base = jspin.DefaultCDN
	}
	base = strings.TrimSuffix(base, "/")
	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	root := base + "/npm/" + pkg + "@" + version + "/"

	// package.json names the manifest; the default is the file at the root.
	var meta struct {
		CustomElements string `json:"customElements"`
	}
	data, err := fetch(ctx, client, root+"package.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("gx wc pin: package.json of %s: %w", opt.Spec, err)
	}
	manifestPath := strings.TrimPrefix(path.Clean("/"+meta.CustomElements), "/")
	if manifestPath == "" {
		manifestPath = "custom-elements.json"
	}
	data, err = fetch(ctx, client, root+manifestPath)
	if err != nil {
		return nil, fmt.Errorf("gx wc pin: %s has no custom elements manifest: %w", opt.Spec, err)
	}
	var mf manifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("gx wc pin: %s of %s: %w", manifestPath, opt.Spec, err)
	}

	// only holds the tags of the filter; missing holds the ones that the
	// manifest has not shown yet.
	only, missing := map[string]bool{}, map[string]bool{}
	for _, tag := range opt.Elements {
		only[tag], missing[tag] = true, true
	}
	set := compiler.ElementSet{Package: pkg, Version: version}
	for _, mod := range mf.Modules {
		for _, decl := range mod.Declarations {
			if decl.TagName == "" || (len(only) > 0 && !only[decl.TagName]) {
				continue
			}
			delete(missing, decl.TagName)
			el := compiler.ElementDef{Tag: decl.TagName, Doc: firstLine(decl.Summary, decl.Description),
				Attributes: []compiler.ElementAttr{}, Events: []string{}, Slots: []string{}}
			// A module path that names a JavaScript file is the module that
			// defines the element. A manifest of source files names a .ts
			// file; the entry of the package then defines the element.
			el.Module = pkg
			if strings.HasSuffix(mod.Path, ".js") || strings.HasSuffix(mod.Path, ".mjs") {
				el.Module = pkg + "/" + path.Join(path.Dir(manifestPath), mod.Path)
			}
			seen := map[string]bool{}
			for _, attr := range decl.Attributes {
				if attr.Name == "" || seen[attr.Name] {
					continue
				}
				seen[attr.Name] = true
				def := compiler.ElementAttr{Name: attr.Name, Doc: firstLine(attr.Description)}
				text := ""
				if attr.Type != nil {
					text = attr.Type.Text
				}
				def.Kind, def.Values = attrKind(text)
				el.Attributes = append(el.Attributes, def)
			}
			seenEvent := map[string]bool{}
			for _, ev := range decl.Events {
				if ev.Name != "" && !seenEvent[ev.Name] {
					seenEvent[ev.Name] = true
					el.Events = append(el.Events, ev.Name)
				}
			}
			seenSlot := map[string]bool{}
			for _, slot := range decl.Slots {
				if slot.Name != "" && !seenSlot[slot.Name] {
					seenSlot[slot.Name] = true
					el.Slots = append(el.Slots, slot.Name)
				}
			}
			set.Elements = append(set.Elements, el)
		}
	}
	if len(missing) > 0 {
		tags := make([]string, 0, len(missing))
		for tag := range missing {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		return nil, fmt.Errorf("gx wc pin: %s has no element %s", opt.Spec, strings.Join(tags, ", "))
	}
	if len(set.Elements) == 0 {
		return nil, fmt.Errorf("gx wc pin: the manifest of %s declares no custom element", opt.Spec)
	}
	sort.Slice(set.Elements, func(i, j int) bool { return set.Elements[i].Tag < set.Elements[j].Tag })

	name := opt.As
	prefix := commonPrefix(set.Elements)
	if name == "" {
		name = prefix
	}
	if name == "" {
		name = goName(pkg[strings.LastIndexByte(pkg, '/')+1:])
	}
	if !goIdent.MatchString(name) {
		return nil, fmt.Errorf("gx wc pin: %q is not a Go package name; give one with --as", name)
	}
	names := map[string]string{}
	for i := range set.Elements {
		el := &set.Elements[i]
		el.Name = componentName(strings.TrimPrefix(el.Tag, prefix+"-"))
		if other, taken := names[el.Name]; taken {
			return nil, fmt.Errorf("gx wc pin: the tags %s and %s both give the name %s", other, el.Tag, el.Name)
		}
		names[el.Name] = el.Tag
	}

	// Pin each module once. A module is <pkg>/<subpath> or the package.
	pinned := map[string]bool{}
	for _, el := range set.Elements {
		if pinned[el.Module] {
			continue
		}
		pinned[el.Module] = true
		spec := pkg + "@" + version + strings.TrimPrefix(el.Module, pkg)
		if _, err := jspin.Pin(ctx, jspin.Options{Dir: opt.Dir, Spec: spec, BaseURL: base, Client: client}); err != nil {
			return nil, err
		}
	}

	out := opt.Out
	if out == "" {
		out = "ui"
	}
	rel := path.Join(filepath.ToSlash(out), name)
	dir := filepath.Join(opt.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, compiler.ElementsFile), append(body, '\n'), 0o644); err != nil {
		return nil, err
	}
	doc := "// Code generated by gx wc pin. DO NOT EDIT.\n\n" +
		"// Package " + name + " holds the typed tags of the custom elements of\n// " + pkg + "@" + version + ". " + compiler.ElementsFile + " describes them.\n" +
		"package " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "elements_gx.go"), []byte(doc), 0o644); err != nil {
		return nil, err
	}
	res := &Result{Package: rel}
	for _, el := range set.Elements {
		res.Tags = append(res.Tags, el.Tag)
	}
	return res, nil
}

func fetch(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gx wc pin: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gx wc pin: fetch %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

var goIdent = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// goName makes a Go package name from a package name part.
func goName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9' && b.Len() > 0) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// commonPrefix returns the first part of the tag names when two or more
// elements share it: "wa" for wa-button and wa-icon.
func commonPrefix(els []compiler.ElementDef) string {
	if len(els) < 2 {
		return ""
	}
	prefix, _, _ := strings.Cut(els[0].Tag, "-")
	for _, el := range els {
		if p, rest, ok := strings.Cut(el.Tag, "-"); !ok || p != prefix || rest == "" {
			return ""
		}
	}
	return prefix
}

// componentName turns a tag name into an exported Go name: "radio-group"
// is RadioGroup.
func componentName(tag string) string {
	var b strings.Builder
	for _, part := range strings.Split(tag, "-") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

func firstLine(texts ...string) string {
	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		line, _, _ := strings.Cut(text, "\n")
		return strings.TrimSpace(line)
	}
	return ""
}

var literalUnion = regexp.MustCompile(`^'[^']*'$|^"[^"]*"$`)

// attrKind maps the type text of a manifest attribute. A union of string
// literals is an enum; a type the import does not know is a string, as
// every HTML attribute is.
func attrKind(text string) (kind string, values []string) {
	var literals []string
	other := false
	for _, part := range strings.Split(text, "|") {
		part = strings.TrimSpace(part)
		switch {
		case part == "" || part == "undefined" || part == "null":
		case literalUnion.MatchString(part):
			literals = append(literals, part[1:len(part)-1])
		default:
			other = true
		}
	}
	plain := strings.TrimSpace(strings.NewReplacer("| undefined", "", "| null", "", "undefined |", "", "null |", "").Replace(text))
	switch {
	case plain == "boolean":
		return compiler.ElementBool, nil
	case plain == "number":
		return compiler.ElementNumber, nil
	case len(literals) > 0 && !other:
		return compiler.ElementEnum, literals
	}
	return compiler.ElementString, nil
}
