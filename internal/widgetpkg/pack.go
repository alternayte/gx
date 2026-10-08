package widgetpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
)

// File is one file of the package.
type File struct {
	Name string
	Data []byte
}

// Package is the npm package of the widgets of an app (REQ-ISL-14): for each
// widget the element file and its type files, and one manifest.
type Package struct {
	Name    string
	Version string
	// Tags are the element names of the widgets. Files holds <tag>.js,
	// <tag>.d.ts and <tag>.react.d.ts for each one.
	Tags  []string
	Files []File
}

// ManifestFile is the name of the custom elements manifest in the package.
const ManifestFile = "custom-elements.json"

var namePattern = regexp.MustCompile(`^(@[a-z0-9~-][a-z0-9._~-]*/)?[a-z0-9~-][a-z0-9._~-]*$`)

// NameProblem says why a name is not a name of an npm package, or "".
func NameProblem(name string) string {
	switch {
	case name == "":
		return "the name is empty"
	case len(name) > 214:
		return "the name has more than 214 characters"
	case !namePattern.MatchString(name):
		return "the name must be lowercase, such as acme-widgets or @acme/widgets"
	}
	return ""
}

// TarballName returns the file name that npm gives the tarball of a version:
// "@acme/widgets" at 1.0.0 gives "acme-widgets-1.0.0.tgz".
func TarballName(name, version string) string {
	name = strings.ReplaceAll(strings.TrimPrefix(name, "@"), "/", "-")
	return name + "-" + version + ".tgz"
}

// exportEntry is one entry of the exports of package.json. The types
// condition comes first: a resolver takes the first condition that matches.
type exportEntry struct {
	Types   string `json:"types,omitempty"`
	Default string `json:"default,omitempty"`
}

// packageJSON is the package.json of the package, with its keys in the order
// of the fields.
type packageJSON struct {
	Name           string                 `json:"name"`
	Version        string                 `json:"version"`
	Type           string                 `json:"type"`
	Files          []string               `json:"files"`
	Exports        map[string]exportEntry `json:"exports"`
	CustomElements string                 `json:"customElements"`
	SideEffects    bool                   `json:"sideEffects"`
}

func (p Package) meta() (packageJSON, error) {
	if problem := NameProblem(p.Name); problem != "" {
		return packageJSON{}, errors.New("the package name " + p.Name + ": " + problem)
	}
	if _, err := ParseVersion(p.Version); err != nil {
		return packageJSON{}, err
	}
	if len(p.Tags) == 0 {
		return packageJSON{}, errors.New("the package has no widget")
	}
	meta := packageJSON{
		Name: p.Name, Version: p.Version, Type: "module",
		Exports:        map[string]exportEntry{"./" + ManifestFile: {Default: "./" + ManifestFile}, "./package.json": {Default: "./package.json"}},
		CustomElements: ManifestFile,
		// The element file defines the custom element when it loads.
		SideEffects: true,
	}
	for _, f := range p.Files {
		meta.Files = append(meta.Files, f.Name)
	}
	sort.Strings(meta.Files)
	for _, tag := range p.Tags {
		meta.Exports["./"+tag] = exportEntry{Types: "./" + tag + ".d.ts", Default: "./" + tag + ".js"}
		meta.Exports["./"+tag+"/react"] = exportEntry{Types: "./" + tag + ".react.d.ts"}
	}
	if len(p.Tags) == 1 {
		// A package of one widget is that widget.
		meta.Exports["."] = meta.Exports["./"+p.Tags[0]]
	}
	return meta, nil
}

// PackageJSON returns the package.json of the package.
func (p Package) PackageJSON() ([]byte, error) {
	meta, err := p.meta()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(meta); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Tarball returns the npm tarball of the package: each file below package/,
// in the order of the names, with no time and no owner. The same files give
// the same bytes, so the integrity of a version does not change with the
// machine or the day.
func (p Package) Tarball() ([]byte, error) {
	meta, err := p.PackageJSON()
	if err != nil {
		return nil, err
	}
	files := append([]File{{Name: "package.json", Data: meta}}, p.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(zw)
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{Name: "package/" + f.Name, Mode: 0o644, Size: int64(len(f.Data)), Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
