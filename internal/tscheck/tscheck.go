// Package tscheck type-checks the TypeScript islands of an app with the
// native TypeScript compiler (REQ-ISL-08). The compiler is the Go port of
// TypeScript: one binary for each platform, with no node. gx downloads the
// pinned release for the host, verifies its sha256 from gx.lock and runs it,
// as it does for Tailwind.
package tscheck

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/jspin"
)

// DefaultVersion is the pinned TypeScript release. From release 7.0 the
// compiler is native code.
const DefaultVersion = "7.0.2"

// sums holds the sha256 of each platform package of DefaultVersion, as the
// npm registry serves it.
var sums = map[string]string{
	"typescript-darwin-arm64-7.0.2.tgz": "902e2fe1cf0799198ef902c6b8c310a450fef629a6baba41d45641ef75c04ebd",
	"typescript-darwin-x64-7.0.2.tgz":   "eba158cb54050f723d5ff781438f33de5640054440bb4f2bd170cfe9bc2eb551",
	"typescript-linux-x64-7.0.2.tgz":    "7ecad6f67377e831856367ab062ef394f21506a611405bf8ac0ff039348637d3",
	"typescript-linux-arm64-7.0.2.tgz":  "c83d931ac9dd7549cde6e71246aa9d6a9812843023df3e277fe3b5dcf41dd0ea",
	"typescript-win32-x64-7.0.2.tgz":    "61fc4e141d2bc687db580e71bbfa63b9c209f0310645d82ca1b457eb3a24fd19",
	"typescript-win32-arm64-7.0.2.tgz":  "0a73534e6ee50cdbb2a29ac48657ca0ad13cf0f424cf63808e4df7baeb87b8be",
}

// DefaultBaseURL is the npm registry. A platform package is at
// <base>/@typescript/typescript-<platform>/-/<asset>.
const DefaultBaseURL = "https://registry.npmjs.org"

// LockEntry pins the compiler in gx.lock.
type LockEntry struct {
	Version string            `json:"version"`
	SHA256  map[string]string `json:"sha256,omitempty"`
}

// DefaultLock returns the built-in pin.
func DefaultLock() LockEntry { return LockEntry{Version: DefaultVersion, SHA256: sums} }

// LoadLock reads the typescript section of gx.lock, or returns the built-in
// pin when the file or the section is absent.
func LoadLock(root string) (LockEntry, error) {
	data, err := os.ReadFile(filepath.Join(root, "gx.lock"))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultLock(), nil
		}
		return LockEntry{}, err
	}
	var raw struct {
		TypeScript *LockEntry `json:"typescript"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return LockEntry{}, fmt.Errorf("typescript: gx.lock: %w", err)
	}
	if raw.TypeScript == nil || raw.TypeScript.Version == "" {
		return DefaultLock(), nil
	}
	if raw.TypeScript.SHA256 == nil && raw.TypeScript.Version == DefaultVersion {
		raw.TypeScript.SHA256 = sums
	}
	return *raw.TypeScript, nil
}

// SaveLock merges the typescript pin into gx.lock without touching other
// sections.
func SaveLock(root string, entry LockEntry) error {
	path := filepath.Join(root, "gx.lock")
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("typescript: gx.lock: %w", err)
		}
	}
	section, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	raw["typescript"] = section
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// HasLock reports whether gx.lock pins the compiler.
func HasLock(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "gx.lock"))
	if err != nil {
		return false
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return false
	}
	_, ok := raw["typescript"]
	return ok
}

// platform returns the platform name of the npm packages.
func platform(goos, goarch string) (string, error) {
	osName := map[string]string{"darwin": "darwin", "linux": "linux", "windows": "win32"}[goos]
	arch := map[string]string{"arm64": "arm64", "amd64": "x64"}[goarch]
	if osName == "" || arch == "" {
		return "", fmt.Errorf("typescript: no native compiler for %s/%s", goos, goarch)
	}
	return osName + "-" + arch, nil
}

// Asset returns the package file name of a GOOS and GOARCH at one version.
func Asset(version, goos, goarch string) (string, error) {
	p, err := platform(goos, goarch)
	if err != nil {
		return "", err
	}
	return "typescript-" + p + "-" + version + ".tgz", nil
}

// Manager fetches and runs the pinned compiler.
type Manager struct {
	// Root is the app directory that holds gx.lock and gx.toml.
	Root string
	// Cache overrides the cache directory.
	Cache string
	// BaseURL overrides the registry, for mirrors (REQ-STY-12) and tests.
	BaseURL string
	// Client is the HTTP client. The default client honours HTTPS_PROXY.
	Client *http.Client
}

func (m *Manager) vendorPath(version, asset string) string {
	return filepath.Join(m.Root, ".gx", "vendor", "typescript", version, asset)
}

func (m *Manager) cacheDir() (string, error) {
	if m.Cache != "" {
		return m.Cache, nil
	}
	if env := os.Getenv("GX_TYPESCRIPT_CACHE"); env != "" {
		return env, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("typescript: cache dir: %w", err)
	}
	return filepath.Join(base, "gx", "typescript"), nil
}

// pinned returns the version, the asset of the host and its sha256.
func (m *Manager) pinned() (version, asset, want string, err error) {
	lock, err := LoadLock(m.Root)
	if err != nil {
		return "", "", "", err
	}
	asset, err = Asset(lock.Version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", "", "", err
	}
	want = lock.SHA256[asset]
	if want == "" {
		return "", "", "", fmt.Errorf("typescript: gx.lock has no sha256 for %s", asset)
	}
	return lock.Version, asset, want, nil
}

// archive returns the path of the verified package file: the vendored one,
// the cached one, or a new download.
func (m *Manager) archive(ctx context.Context) (path, want string, err error) {
	version, asset, want, err := m.pinned()
	if err != nil {
		return "", "", err
	}
	// A vendored package wins and works with the network off (REQ-STY-12).
	vendored := m.vendorPath(version, asset)
	if got, err := hashFile(vendored); err == nil {
		if got != want {
			return "", "", fmt.Errorf("typescript: vendored %s sha256 = %s, gx.lock pins %s", vendored, got, want)
		}
		return vendored, want, nil
	}
	dir, err := m.cacheDir()
	if err != nil {
		return "", "", err
	}
	path = filepath.Join(dir, version, asset)
	if got, err := hashFile(path); err == nil && got == want {
		return path, want, nil
	}
	url, err := m.downloadURL(version, asset)
	if err != nil {
		return "", "", err
	}
	if err := m.download(ctx, url, path, want); err != nil {
		return "", "", err
	}
	return path, want, nil
}

// downloadURL resolves the download URL: BaseURL, then
// GX_TYPESCRIPT_BASE_URL, then the tsgo mirror of gx.toml, then the npm
// registry (REQ-STY-12).
func (m *Manager) downloadURL(version, asset string) (string, error) {
	base := m.BaseURL
	if base == "" {
		base = os.Getenv("GX_TYPESCRIPT_BASE_URL")
	}
	if base == "" {
		cfg, err := gxconfig.Load(m.Root)
		if err != nil {
			return "", err
		}
		base = cfg.Mirror("tsgo")
	}
	if base == "" {
		base = DefaultBaseURL
	}
	if strings.Contains(base, "{version}") || strings.Contains(base, "{asset}") {
		base = strings.ReplaceAll(base, "{version}", version)
		return strings.ReplaceAll(base, "{asset}", asset), nil
	}
	pkg := strings.TrimSuffix(asset, "-"+version+".tgz")
	return strings.TrimSuffix(base, "/") + "/@typescript/" + pkg + "/-/" + asset, nil
}

// download fetches url, verifies its sha256 and moves it to path.
func (m *Manager) download(ctx context.Context, url, path, want string) error {
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("typescript: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("typescript: download %s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".typescript-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("typescript: download %s: %w", url, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("typescript: %s sha256 = %s, want %s", url, got, want)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		// A concurrent installer may have won the rename.
		if got, hashErr := hashFile(path); hashErr == nil && got == want {
			return nil
		}
		return err
	}
	return nil
}

// Vendor stores the pinned package in .gx/vendor for offline builds
// (REQ-STY-12).
func (m *Manager) Vendor(ctx context.Context) (string, error) {
	src, want, err := m.archive(ctx)
	if err != nil {
		return "", err
	}
	version, asset, _, err := m.pinned()
	if err != nil {
		return "", err
	}
	dst := m.vendorPath(version, asset)
	if got, err := hashFile(dst); err == nil && got == want {
		return dst, nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	return dst, os.WriteFile(dst, data, 0o644)
}

// Ensure returns the path of the compiler binary. It unpacks the verified
// package next to the cache entry one time; the directory name holds the
// sha256, so a different package never reuses it.
func (m *Manager) Ensure(ctx context.Context) (string, error) {
	archive, want, err := m.archive(ctx)
	if err != nil {
		return "", err
	}
	cache, err := m.cacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "unpacked", want)
	bin := filepath.Join(dir, "lib", "tsc")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(filepath.Join(dir, ".complete")); err == nil {
		return bin, nil
	}
	tmp, err := os.MkdirTemp(cache, ".unpack-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := unpack(archive, tmp); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, ".complete"), nil, 0o644); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dir); err != nil {
		// A concurrent check may have won the rename.
		if _, statErr := os.Stat(filepath.Join(dir, ".complete")); statErr != nil {
			return "", err
		}
	}
	return bin, nil
}

// unpack writes the lib directory of a platform package into dir: the
// compiler binary and the lib.*.d.ts files next to it.
func unpack(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("typescript: %s: %w", archive, err)
	}
	tr := tar.NewReader(gz)
	found := false
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("typescript: %s: %w", archive, err)
		}
		name, ok := strings.CutPrefix(hdr.Name, "package/lib/")
		// A file directly in lib only: no path of the package can leave dir.
		if !ok || hdr.Typeflag != tar.TypeReg || name == "" || strings.ContainsAny(name, `/\`) || name == ".." {
			continue
		}
		mode := os.FileMode(0o644)
		if name == "tsc" || name == "tsc.exe" {
			mode = 0o755
			found = true
		}
		dst := filepath.Join(dir, "lib", name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("typescript: %s holds no compiler", archive)
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// options are the compiler options of an app with no tsconfig.json. They
// match what the island bundler accepts: ES modules for a browser.
var options = []string{
	"--noEmit", "--strict",
	"--target", "es2022", "--module", "esnext", "--moduleResolution", "bundler",
	"--lib", "es2022,dom,dom.iterable",
	"--skipLibCheck", "--pretty", "false",
}

// errorLine is one error of the compiler with --pretty false.
var errorLine = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\): error (TS\d+): (.*)$`)

// Check type-checks every island of the app and returns one GX6005 for each
// TypeScript error. An app with no island runs no compiler and needs no
// download. An app with a tsconfig.json is checked as that project.
func (m *Manager) Check(ctx context.Context) ([]compiler.Diagnostic, error) {
	root, err := filepath.Abs(m.Root)
	if err != nil {
		return nil, err
	}
	refs := compiler.Islands(root)
	// The TypeScript client of gx api is checked with the islands
	// (REQ-ACT-20).
	client := filepath.Join(root, compiler.APIDir, "client.ts")
	if !fileExists(client) {
		client = ""
	}
	if len(refs) == 0 && client == "" {
		return nil, nil
	}
	bin, err := m.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	var args []string
	if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
		args = []string{"--project", root, "--noEmit", "--pretty", "false"}
	} else {
		args = append(args, options...)
		for _, ref := range refs {
			args = append(args, ref.File)
		}
		if client != "" {
			args = append(args, client)
		}
		// The declarations of the pinned packages (REQ-ISL-07).
		if types := filepath.Join(root, filepath.FromSlash(jspin.TypesFile)); fileExists(types) {
			args = append(args, types)
		}
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = root
	out, runErr := cmd.CombinedOutput()
	diags := parseErrors(root, string(out))
	if runErr != nil && len(diags) == 0 {
		return nil, fmt.Errorf("typescript: %w\n%s", runErr, out)
	}
	return diags, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseErrors reads the output of the compiler. A line that starts with
// white space continues the message of the error before it.
func parseErrors(root, out string) []compiler.Diagnostic {
	var diags []compiler.Diagnostic
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if m := errorLine.FindStringSubmatch(line); m != nil {
			file := m[1]
			if !filepath.IsAbs(file) {
				file = filepath.Join(root, filepath.FromSlash(file))
			}
			lineNo, _ := strconv.Atoi(m[2])
			col, _ := strconv.Atoi(m[3])
			diags = append(diags, compiler.Diagnostic{
				Code: compiler.CodeIslandTypeScript,
				File: file,
				Line: lineNo,
				Col:  col,
				Msg:  m[4] + ": " + m[5],
			})
			continue
		}
		if len(diags) > 0 && strings.HasPrefix(line, " ") && strings.TrimSpace(line) != "" {
			diags[len(diags)-1].Msg += " " + strings.TrimSpace(line)
		}
	}
	return diags
}

// App is the whole check of an app, as `gx check` runs it: the check of the
// compiler, then the TypeScript check of the islands. The TypeScript check
// needs the generated props files, so it does not run while the compiler
// reports a stale file or an island with no props.
func App(ctx context.Context, dir string, opt compiler.CheckOptions) ([]compiler.Diagnostic, error) {
	diags := compiler.CheckApp(dir, opt)
	for _, d := range diags {
		switch d.Code {
		case compiler.CodeStale, compiler.CodeIslandProps, compiler.CodeIslandType:
			return diags, nil
		}
	}
	ts, err := (&Manager{Root: dir}).Check(ctx)
	if err != nil {
		return diags, err
	}
	return append(diags, ts...), nil
}
