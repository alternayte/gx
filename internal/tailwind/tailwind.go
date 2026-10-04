// Package tailwind manages the pinned Tailwind CSS standalone binary
// (REQ-STY-01). It downloads the binary for the host platform, verifies the
// sha256 from gx.lock and runs it. Node is never involved.
package tailwind

import (
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
)

// DefaultVersion is the pinned Tailwind release.
const DefaultVersion = "v4.3.3"

// sums holds the published sha256 of every release asset of DefaultVersion.
var sums = map[string]string{
	"tailwindcss-linux-arm64":      "55fd0b241214eff3de1e8ee4f22796662f2d2e7a49bcfca7477cfd0bac398195",
	"tailwindcss-linux-arm64-musl": "71ea4be79c9de9827545682df3e040053fb535d37c71ed2cfdedf9385a0868e0",
	"tailwindcss-linux-x64":        "dc61b3ac6b8c9ca874c0cc4c57b2409791a64c5540404ca5f5367360babc313a",
	"tailwindcss-linux-x64-musl":   "a04d34ceacc8f52cbe8920ad846cdeb61d3d0021dba32db0d1f77c9d9fad7a6c",
	"tailwindcss-macos-arm64":      "cdf646702987a743464dff4d9c60fd4480d1c1e73dd819a9a67f1078815dce9d",
	"tailwindcss-macos-x64":        "7922e0953f2110c05976e3bf58f14e643d90427575e766b7d433f5f80cbee7e1",
	"tailwindcss-windows-x64.exe":  "e0e260ce048014e9268f6237ff18f8ccf02cef521cbd0ae04e82c2cdf7aa3955",
}

// DefaultBaseURL is the release download root.
const DefaultBaseURL = "https://github.com/tailwindlabs/tailwindcss/releases/download"

// Lock is the gx.lock shape. The file is machine-written.
type Lock struct {
	Tailwind LockEntry `json:"tailwind"`
}

// LockEntry pins one tool.
type LockEntry struct {
	Version string            `json:"version"`
	SHA256  map[string]string `json:"sha256,omitempty"`
}

// DefaultLock returns the built-in pin of the official release.
func DefaultLock() Lock {
	return Lock{Tailwind: LockEntry{Version: DefaultVersion, SHA256: sums}}
}

// LoadLock reads gx.lock from a module root, or returns the default pin when
// the file is absent.
func LoadLock(root string) (Lock, error) {
	data, err := os.ReadFile(filepath.Join(root, "gx.lock"))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultLock(), nil
		}
		return Lock{}, err
	}
	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return Lock{}, fmt.Errorf("tailwind: gx.lock: %w", err)
	}
	if lock.Tailwind.Version == "" {
		lock.Tailwind.Version = DefaultVersion
	}
	if lock.Tailwind.SHA256 == nil {
		lock.Tailwind.SHA256 = sums
	}
	return lock, nil
}

// Manager fetches and runs the pinned binary.
type Manager struct {
	// Root is the module root that holds gx.lock.
	Root string
	// Cache overrides the binary cache directory.
	Cache string
	// BaseURL overrides the release download root, for mirrors (REQ-STY-12)
	// and tests.
	BaseURL string
	// Client is the HTTP client, or http.DefaultClient.
	Client *http.Client
}

// Asset returns the release asset name of a GOOS and GOARCH.
func Asset(goos, goarch string) (string, error) {
	switch goos {
	case "darwin":
		switch goarch {
		case "arm64":
			return "tailwindcss-macos-arm64", nil
		case "amd64":
			return "tailwindcss-macos-x64", nil
		}
	case "linux":
		switch goarch {
		case "arm64":
			return "tailwindcss-linux-arm64", nil
		case "amd64":
			return "tailwindcss-linux-x64", nil
		}
	case "windows":
		if goarch == "amd64" {
			return "tailwindcss-windows-x64.exe", nil
		}
	}
	return "", fmt.Errorf("tailwind: no standalone binary for %s/%s", goos, goarch)
}

// Ensure returns the path of the verified binary, downloading it when the
// cache is cold (REQ-STY-01).
func (m *Manager) Ensure(ctx context.Context) (string, error) {
	lock, err := LoadLock(m.Root)
	if err != nil {
		return "", err
	}
	asset, err := Asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	want, ok := lock.Tailwind.SHA256[asset]
	if !ok || want == "" {
		return "", fmt.Errorf("tailwind: gx.lock has no sha256 for %s", asset)
	}
	version := lock.Tailwind.Version
	if version == "" {
		version = DefaultVersion
	}
	dir := m.Cache
	if dir == "" {
		if env := os.Getenv("GX_TAILWIND_CACHE"); env != "" {
			dir = env
		} else {
			base, err := os.UserCacheDir()
			if err != nil {
				return "", fmt.Errorf("tailwind: cache dir: %w", err)
			}
			dir = filepath.Join(base, "gx", "tailwind")
		}
	}
	path := filepath.Join(dir, version, asset)
	if got, err := hashFile(path); err == nil && got == want {
		return path, nil
	}
	baseURL := m.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	url := strings.TrimSuffix(baseURL, "/") + "/" + version + "/" + asset
	if err := m.download(ctx, url, path, want); err != nil {
		return "", err
	}
	return path, nil
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
		return fmt.Errorf("tailwind: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tailwind: download %s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tailwind-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("tailwind: download %s: %w", url, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("tailwind: %s sha256 = %s, want %s", url, got, want)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
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

// Run runs the pinned binary with args (REQ-STY-01). It returns the combined
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
		return out, fmt.Errorf("tailwind: %v: %w\n%s", args, err, out)
	}
	return out, nil
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
