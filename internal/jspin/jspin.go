// Package jspin vendors pre-bundled ESM builds of npm packages for the
// islands of an app (REQ-ISL-07). `gx pin <pkg>@<version>` fetches the build
// and the builds it imports from the ESM CDN, stores them in js/vendor/ and
// records the integrity of each file in gx.lock. The island bundler resolves
// a bare import to the vendored file. Node is never involved.
package jspin

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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

	"github.com/alternayte/gx/internal/gxconfig"
)

// DefaultCDN is the ESM CDN. Its /npm/<pkg>@<version>/+esm address answers
// with one ES module that imports its dependencies by the same kind of
// address.
const DefaultCDN = "https://cdn.jsdelivr.net"

// VendorDir is the directory of the vendored files, relative to the app.
const VendorDir = "js/vendor"

// TypesFile declares each pinned package for the TypeScript check.
const TypesFile = VendorDir + "/pins.d.ts"

// maxFiles bounds one pin: a package with more modules than this is not a
// pre-bundled build.
const maxFiles = 400

// Lock is the "js" section of gx.lock.
type Lock struct {
	// Pins maps an import specifier ("echarts", "echarts/core") to its pin.
	Pins map[string]Entry `json:"pins"`
	// Files maps each vendored file to its integrity: "sha256-" and the
	// base64 form of the hash.
	Files map[string]string `json:"files"`
}

// Entry is one pinned import specifier.
type Entry struct {
	Version string `json:"version"`
	// File is the vendored entry file, relative to the app.
	File string `json:"file"`
}

// module is one ES module of the CDN.
type module struct {
	name    string // "d3-scale" or "@scope/name"
	version string
	subpath string // "" or "core"
}

// specifier is the import specifier of the module in an island file.
func (m module) specifier() string {
	if m.subpath == "" {
		return m.name
	}
	return m.name + "/" + m.subpath
}

// url is the address path of the module on the CDN.
func (m module) url() string {
	p := "/npm/" + m.name + "@" + m.version
	if m.subpath != "" {
		p += "/" + m.subpath
	}
	return p + "/+esm"
}

// file is the vendored file of the module, relative to the app.
func (m module) file() string {
	p := VendorDir + "/" + m.name + "@" + m.version
	if m.subpath != "" {
		p += "/" + m.subpath
	}
	return p + ".js"
}

var specPattern = regexp.MustCompile(`^((?:@[a-z0-9][a-z0-9._~-]*/)?[a-z0-9][a-z0-9._~-]*)@([A-Za-z0-9][A-Za-z0-9.+_-]*)((?:/[A-Za-z0-9._-]+)*)$`)

// parseSpec reads <pkg>@<version> with an optional subpath.
func parseSpec(spec string) (module, error) {
	m := specPattern.FindStringSubmatch(spec)
	if m == nil {
		return module{}, fmt.Errorf("gx pin: %q is not <pkg>@<version>, for example echarts@5.5.1", spec)
	}
	mod := module{name: m[1], version: m[2], subpath: strings.TrimPrefix(m[3], "/")}
	if err := mod.check(); err != nil {
		return module{}, err
	}
	return mod, nil
}

// check refuses a module whose file would be outside the vendor directory:
// a name, a version or a subpath with a "." or ".." part, or with a
// backslash. The text of a fetched module names its imports, so each
// import gets this check too.
func (m module) check() error {
	for _, part := range strings.Split(m.name+"/"+m.version+"/"+m.subpath, "/") {
		if part == "." || part == ".." || strings.ContainsAny(part, `\:`) {
			return fmt.Errorf("gx pin: the module path %q has a part that leaves the vendor directory", m.name+"@"+m.version+"/"+m.subpath)
		}
	}
	return nil
}

// cdnImport matches the address of a module of the CDN in a string literal.
var cdnImport = regexp.MustCompile(`(["'])/npm/((?:@[^/"'@]+/)?[^/"'@]+)@([^/"']+)((?:/[^"']+?)?)/\+esm(["'])`)

// sourceMapLine is the last line of a CDN build. The map is not vendored.
var sourceMapLine = regexp.MustCompile(`(?m)^//# sourceMappingURL=.*\n?`)

// Options configure one pin.
type Options struct {
	// Dir is the app directory.
	Dir string
	// Spec is <pkg>@<version>, with an optional subpath.
	Spec string
	// BaseURL overrides the CDN. The default is the esm mirror of gx.toml,
	// then DefaultCDN.
	BaseURL string
	// Client is the HTTP client. The default client honours HTTPS_PROXY.
	Client *http.Client
}

// Result reports one pin.
type Result struct {
	Specifier string
	Version   string
	// Files lists the vendored files of the pin, relative to the app.
	Files []string
}

// Pin fetches a package and every module it imports, stores them and
// records them in gx.lock.
func Pin(ctx context.Context, opt Options) (*Result, error) {
	root, err := parseSpec(opt.Spec)
	if err != nil {
		return nil, err
	}
	base := opt.BaseURL
	if base == "" {
		if cfg, err := gxconfig.Load(opt.Dir); err == nil {
			base = cfg.Mirror("esm")
		}
	}
	if base == "" {
		base = DefaultCDN
	}
	base = strings.TrimSuffix(base, "/")
	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	lock, err := LoadLock(opt.Dir)
	if err != nil {
		return nil, err
	}

	// Fetch every module before the first write, so a failed pin leaves
	// the app as it was.
	contents := map[string][]byte{}
	queue := []module{root}
	seen := map[string]bool{root.file(): true}
	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]
		if len(contents) >= maxFiles {
			return nil, fmt.Errorf("gx pin: %s imports more than %d modules", opt.Spec, maxFiles)
		}
		body, err := fetch(ctx, client, base+m.url())
		if err != nil {
			return nil, err
		}
		body = sourceMapLine.ReplaceAll(body, nil)
		from := m.file()
		var bad error
		body = cdnImport.ReplaceAllFunc(body, func(match []byte) []byte {
			parts := cdnImport.FindSubmatch(match)
			dep := module{name: string(parts[2]), version: string(parts[3]), subpath: strings.TrimPrefix(string(parts[4]), "/")}
			if err := dep.check(); err != nil {
				bad = err
				return match
			}
			if !seen[dep.file()] {
				seen[dep.file()] = true
				queue = append(queue, dep)
			}
			rel, err := filepath.Rel(filepath.FromSlash(path.Dir(from)), filepath.FromSlash(dep.file()))
			if err != nil {
				return match
			}
			rel = filepath.ToSlash(rel)
			if !strings.HasPrefix(rel, ".") {
				rel = "./" + rel
			}
			return []byte(string(parts[1]) + rel + string(parts[5]))
		})
		if bad != nil {
			return nil, bad
		}
		contents[from] = body
	}

	res := &Result{Specifier: root.specifier(), Version: root.version}
	for file := range contents {
		res.Files = append(res.Files, file)
	}
	sort.Strings(res.Files)
	for _, file := range res.Files {
		full := filepath.Join(opt.Dir, filepath.FromSlash(file))
		// A second guard after module.check: no file leaves the vendor
		// directory.
		if vendor := filepath.Join(opt.Dir, filepath.FromSlash(VendorDir)) + string(filepath.Separator); !strings.HasPrefix(full, vendor) {
			return nil, fmt.Errorf("gx pin: %s is outside %s", file, VendorDir)
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(full, contents[file], 0o644); err != nil {
			return nil, err
		}
		lock.Files[file] = integrity(contents[file])
	}
	lock.Pins[root.specifier()] = Entry{Version: root.version, File: root.file()}
	if err := SaveLock(opt.Dir, lock); err != nil {
		return nil, err
	}
	if err := writeTypes(opt.Dir, lock); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("gx pin: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gx pin: fetch %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// integrity returns the subresource integrity form of the sha256 of data.
func integrity(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

// writeTypes writes the declaration file of the pins. A pre-bundled build
// has no type declarations, so each pinned package has the type any. An app
// that wants the types of a package installs it with a package manager.
func writeTypes(dir string, lock Lock) error {
	var b strings.Builder
	b.WriteString("// Code generated by gx pin. DO NOT EDIT.\n//\n")
	b.WriteString("// A pinned package is a pre-bundled build with no type declarations, so\n")
	b.WriteString("// its exports have the type any.\n")
	for _, spec := range sortedKeys(lock.Pins) {
		data, _ := json.Marshal(spec)
		b.WriteString("declare module " + string(data) + ";\n")
	}
	full := filepath.Join(dir, filepath.FromSlash(TypesFile))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(b.String()), 0o644)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// LoadLock reads the js section of gx.lock. An app with no pin gets an
// empty lock.
func LoadLock(root string) (Lock, error) {
	lock := Lock{Pins: map[string]Entry{}, Files: map[string]string{}}
	data, err := os.ReadFile(filepath.Join(root, "gx.lock"))
	if err != nil {
		if os.IsNotExist(err) {
			return lock, nil
		}
		return lock, err
	}
	var raw struct {
		JS *Lock `json:"js"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return lock, fmt.Errorf("gx pin: gx.lock: %w", err)
	}
	if raw.JS != nil {
		if raw.JS.Pins != nil {
			lock.Pins = raw.JS.Pins
		}
		if raw.JS.Files != nil {
			lock.Files = raw.JS.Files
		}
	}
	return lock, nil
}

// SaveLock merges the js section into gx.lock without touching other
// sections.
func SaveLock(root string, lock Lock) error {
	file := filepath.Join(root, "gx.lock")
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("gx pin: gx.lock: %w", err)
		}
	}
	section, err := json.Marshal(lock)
	if err != nil {
		return err
	}
	raw["js"] = section
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

// Verify checks every vendored file against gx.lock (SI-10). A missing or
// changed file is an error that stops the build.
func Verify(root string, lock Lock) error {
	for _, file := range sortedKeys(lock.Files) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return fmt.Errorf("gx: the vendored file %s is in gx.lock and not on the disk; run gx pin again", file)
		}
		if got := integrity(data); got != lock.Files[file] {
			return fmt.Errorf("gx: the vendored file %s does not match gx.lock (want %s, got %s); run gx pin again or restore the file", file, lock.Files[file], got)
		}
	}
	return nil
}

// UsesNodeModules reports whether the app resolves imports as node does:
// it has a package.json and a node_modules directory. The bundler then
// leaves every import to its own resolution.
func UsesNodeModules(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(root, "node_modules"))
	return err == nil && info.IsDir()
}
