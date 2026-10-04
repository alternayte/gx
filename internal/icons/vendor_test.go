package icons

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestREQ_STY_12_IconVendorOffline covers the icon mirror and the vendored
// pack: after gx vendor the pin works with the network off (REQ-STY-12).
func TestREQ_STY_12_IconVendorOffline(t *testing.T) {
	pack := `{"prefix":"x","width":24,"height":24,"icons":{"box":{"body":"<rect/>"}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(pack))
	}))
	defer srv.Close()
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/gx.toml", []byte("[mirrors]\nicons = \""+srv.URL+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := Options{Dir: dir, Set: "x", Version: "1.0"}
	if _, err := Vendor(context.Background(), opt); err != nil {
		t.Fatalf("Vendor: %v", err)
	}
	if _, err := os.Stat(dir + "/.gx/vendor/icons/x.json"); err != nil {
		t.Fatalf("vendored pack: %v", err)
	}
	// Offline: the env base points at a dead server, the vendor wins.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusBadGateway)
	}))
	defer dead.Close()
	t.Setenv("GX_ICONIFY_BASE", dead.URL)
	if err := Pin(context.Background(), opt); err != nil {
		t.Fatalf("offline Pin: %v", err)
	}
	// A tampered vendored pack stops the pin (SI-10).
	if err := os.WriteFile(dir+"/.gx/vendor/icons/x.json", []byte(pack+" "), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Pin(context.Background(), opt); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("tampered pack error = %v", err)
	}
}
