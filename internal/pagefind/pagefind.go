// Package pagefind manages the pinned Pagefind standalone binary
// (REQ-CNT-07). It downloads the release for the host platform, verifies the
// sha256 from gx.lock, unpacks it and runs it over a site directory. Node is
// never involved.
package pagefind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alternayte/gx/internal/gxconfig"
)

// DefaultVersion is the pinned Pagefind release.
const DefaultVersion = "v1.5.2"

// sums holds the published sha256 of every release asset of DefaultVersion.
var sums = map[string]string{
	"pagefind-v1.5.2-aarch64-apple-darwin.tar.gz":       "7286f394a349bd37677d44a65a20078a02b1747da0b0814d83403bf86be17abe",
	"pagefind-v1.5.2-x86_64-apple-darwin.tar.gz":        "26f51b4ba921897142338fb13b836420696e251fe197b1782ea7981de311156d",
	"pagefind-v1.5.2-x86_64-unknown-linux-musl.tar.gz":  "afb824a9e7f64905a934900481cea5be679c03975e527329e0e5e6cc70f5feda",
	"pagefind-v1.5.2-aarch64-unknown-linux-musl.tar.gz": "f50ec608bcbf431cebd84e0efa3a5b041ee63df2ad81f138023e0cfd2f509424",
	"pagefind-v1.5.2-x86_64-pc-windows-msvc.tar.gz":     "fab125d5e8e2d3481ffe7d36dec6e101f54a6581cd51cf5e2e09220d4bc78e9c",
}

// DefaultBaseURL is the release download root.
const DefaultBaseURL = "https://github.com/Pagefind/pagefind/releases/download"

// Lock is the pagefind section of gx.lock.
type Lock struct {
	Pagefind LockEntry `json:"pagefind"`
}

// LockEntry pins one tool.
type LockEntry struct {
	Version string            `json:"version"`
	SHA256  map[string]string `json:"sha256,omitempty"`
}

// DefaultLock returns the built-in pin of the official release.
func DefaultLock() Lock {
	return Lock{Pagefind: LockEntry{Version: DefaultVersion, SHA256: sums}}
}

// LoadLock reads the pagefind section of gx.lock, or returns the default
// pin when the file or the section is absent.
func LoadLock(root string) (Lock, error) {
	path := filepath.Join(root, "gx.lock")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultLock(), nil
		}
		return Lock{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Lock{}, fmt.Errorf("pagefind: gx.lock: %w", err)
	}
	lock := DefaultLock()
	if section, ok := raw["pagefind"]; ok {
		var entry LockEntry
		if err := json.Unmarshal(section, &entry); err != nil {
			return Lock{}, fmt.Errorf("pagefind: gx.lock pagefind: %w", err)
		}
		if entry.Version != "" {
			lock.Pagefind.Version = entry.Version
		}
		if entry.SHA256 != nil {
			lock.Pagefind.SHA256 = entry.SHA256
		}
	}
	return lock, nil
}

// SaveLock merges the pagefind pin into gx.lock without touching other
// sections.
func SaveLock(root string, entry LockEntry) error {
	path := filepath.Join(root, "gx.lock")
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("pagefind: gx.lock: %w", err)
		}
	}
	section, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	raw["pagefind"] = section
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// Asset returns the release asset name of a GOOS and GOARCH at the
// default version.
func Asset(goos, goarch string) (string, error) {
	return assetFor(DefaultVersion, goos, goarch)
}

// assetFor returns the release asset name at one version.
func assetFor(version, goos, goarch string) (string, error) {
	target := ""
	switch goos {
	case "darwin":
		switch goarch {
		case "arm64":
			target = "aarch64-apple-darwin"
		case "amd64":
			target = "x86_64-apple-darwin"
		}
	case "linux":
		switch goarch {
		case "arm64":
			target = "aarch64-unknown-linux-musl"
		case "amd64":
			target = "x86_64-unknown-linux-musl"
		}
	case "windows":
		switch goarch {
		case "amd64":
			target = "x86_64-pc-windows-msvc"
		}
	}
	if target == "" {
		return "", fmt.Errorf("pagefind: no standalone binary for %s/%s", goos, goarch)
	}
	return "pagefind-" + version + "-" + target + ".tar.gz", nil
}

// Manager fetches, unpacks and runs the pinned binary.
type Manager struct {
	// Root is the module root that holds gx.lock and gx.toml.
	Root string
	// Cache overrides the binary cache directory.
	Cache string
	// BaseURL overrides the release download root, for mirrors (REQ-STY-12)
	// and tests.
	BaseURL string
	// Client is the HTTP client, or a client with a long timeout.
	Client *http.Client
}

// Ensure returns the path of the verified binary, downloading and unpacking
// it when the cache is cold (REQ-CNT-07).
func (m *Manager) Ensure(ctx context.Context) (string, error) {
	lock, err := LoadLock(m.Root)
	if err != nil {
		return "", err
	}
	version := lock.Pagefind.Version
	if version == "" {
		version = DefaultVersion
	}
	asset, err := assetFor(version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	want, ok := lock.Pagefind.SHA256[asset]
	if !ok || want == "" {
		return "", fmt.Errorf("pagefind: gx.lock has no sha256 for %s", asset)
	}
	name := binaryName()
	// A vendored binary wins and works with the network off (REQ-STY-12).
	vendored := m.vendorPath(version, name)
	if got, err := hashFile(vendored); err == nil {
		if got != want {
			return "", fmt.Errorf("pagefind: vendored %s sha256 = %s, gx.lock pins %s", vendored, got, want)
		}
		return vendored, nil
	}
	dir := m.Cache
	if dir == "" {
		if env := os.Getenv("GX_PAGEFIND_CACHE"); env != "" {
			dir = env
		} else {
			base, err := os.UserCacheDir()
			if err != nil {
				return "", fmt.Errorf("pagefind: cache dir: %w", err)
			}
			dir = filepath.Join(base, "gx", "pagefind")
		}
	}
	path := filepath.Join(dir, version, name)
	if got, err := hashFile(path); err == nil && got == want {
		return path, nil
	}
	url, err := m.downloadURL(version, asset)
	if err != nil {
		return "", err
	}
	if err := m.unpack(ctx, url, path, want); err != nil {
		return "", err
	}
	return path, nil
}

// vendorPath returns the vendored binary path.
func (m *Manager) vendorPath(version, name string) string {
	return filepath.Join(m.Root, ".gx", "vendor", "pagefind", version, name)
}

// Vendor downloads the pinned binary into .gx/vendor for offline builds
// (REQ-STY-12).
func (m *Manager) Vendor(ctx context.Context) (string, error) {
	path, err := m.Ensure(ctx)
	if err != nil {
		return "", err
	}
	lock, err := LoadLock(m.Root)
	if err != nil {
		return "", err
	}
	version := lock.Pagefind.Version
	if version == "" {
		version = DefaultVersion
	}
	asset, err := assetFor(version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	want, ok := lock.Pagefind.SHA256[asset]
	if !ok {
		return "", fmt.Errorf("pagefind: gx.lock has no sha256 for %s", asset)
	}
	dst := m.vendorPath(version, binaryName())
	if got, err := hashFile(dst); err == nil && got == want {
		return dst, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return "", err
	}
	return dst, nil
}

// binaryName is the unpacked binary name.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "pagefind.exe"
	}
	return "pagefind"
}

// downloadURL resolves the download URL: BaseURL, then
// GX_PAGEFIND_BASE_URL, then the gx.toml mirror, then the official release
// root (REQ-STY-12).
func (m *Manager) downloadURL(version, asset string) (string, error) {
	base := m.BaseURL
	if base == "" {
		base = os.Getenv("GX_PAGEFIND_BASE_URL")
	}
	if base == "" {
		cfg, err := gxconfig.Load(m.Root)
		if err != nil {
			return "", err
		}
		base = cfg.Mirror("pagefind")
	}
	if base == "" {
		base = DefaultBaseURL
	}
	if strings.Contains(base, "{version}") || strings.Contains(base, "{asset}") {
		base = strings.ReplaceAll(base, "{version}", version)
		return strings.ReplaceAll(base, "{asset}", asset), nil
	}
	return strings.TrimSuffix(base, "/") + "/" + version + "/" + asset, nil
}

// unpack fetches the tarball, verifies its sha256 and extracts the binary.
func (m *Manager) unpack(ctx context.Context, url, path, want string) error {
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
		return fmt.Errorf("pagefind: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pagefind: download %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("pagefind: download %s: %w", url, err)
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("pagefind: %s sha256 = %s, want %s", url, hex.EncodeToString(got[:]), want)
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("pagefind: %s: %w", url, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("pagefind: %s: %w", url, err)
		}
		if filepath.Base(hdr.Name) != binaryName() {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(path), ".pagefind-*")
		if err != nil {
			return err
		}
		defer func() {
			_ = os.Remove(tmp.Name())
		}()
		if _, err := io.Copy(tmp, tr); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("pagefind: extract: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		if err := os.Chmod(tmp.Name(), 0o755); err != nil {
			return err
		}
		return os.Rename(tmp.Name(), path)
	}
	return fmt.Errorf("pagefind: %s holds no %s", url, binaryName())
}

// Run runs the pinned binary with args (REQ-CNT-07). It returns the combined
// output.
func (m *Manager) Run(ctx context.Context, args ...string) ([]byte, error) {
	path, err := m.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = m.Root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("pagefind: %v: %w\n%s", args, err, out)
	}
	return out, nil
}

// Index runs Pagefind over siteDir. The index lands in
// siteDir/pagefind (REQ-CNT-07).
func (m *Manager) Index(ctx context.Context, siteDir string) error {
	_, err := m.Run(ctx, "--site", siteDir)
	return err
}

// hashFile returns the hex sha256 of a file.
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

// HasLock reports whether gx.lock holds a pagefind section.
func HasLock(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "gx.lock"))
	if err != nil {
		return false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	_, ok := raw["pagefind"]
	return ok
}
