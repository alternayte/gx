package gx

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestREQ_ISL_03_IslandNamesItsEntryFile(t *testing.T) {
	SetIslands(IslandBundle{
		Entries: map[string]string{"app/dash/Chart": "app/dash/Chart-ABCD1234.js"},
		Files:   map[string]string{"app/dash/Chart-ABCD1234.js": "export default()=>{}", "app/dash/Chart-ABCD1234.js.map": "{}"},
	})
	defer SetIslands(IslandBundle{})
	got := String(Island("app/dash/Chart", `{}`))
	if want := `<gx-island name="app/dash/Chart" props="{}" src="/_gx/islands/app/dash/Chart-ABCD1234.js"></gx-island>`; got != want {
		t.Fatalf("island = %s, want %s", got, want)
	}

	app := New(Config{})
	for path, wantType := range map[string]string{
		"/_gx/islands/app/dash/Chart-ABCD1234.js":     "text/javascript; charset=utf-8",
		"/_gx/islands/app/dash/Chart-ABCD1234.js.map": "application/json; charset=utf-8",
	} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || rec.Header().Get("Content-Type") != wantType {
			t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Header().Get("Content-Type"))
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
			t.Fatalf("Cache-Control = %q", cc)
		}
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/_gx/islands/other.js", nil))
	if rec.Code != 404 {
		t.Fatalf("unknown file = %d, want 404", rec.Code)
	}
}

// A mounted app names the island file under its base path (REQ-RTE-18).
func TestREQ_ISL_03_IslandSrcHasTheBasePath(t *testing.T) {
	SetIslands(IslandBundle{Entries: map[string]string{"app/Chart": "app/Chart-ABCD1234.js"}})
	defer SetIslands(IslandBundle{})
	SetBasePath("/shop")
	defer SetBasePath("")
	if got := String(Island("app/Chart", `{}`)); !strings.Contains(got, `src="/shop/_gx/islands/app/Chart-ABCD1234.js"`) {
		t.Fatalf("island = %s", got)
	}
}

// In dev, an island outside the bundle stops the render: the browser has
// no file to load.
func TestREQ_ISL_03_UnknownIslandPanicsInDev(t *testing.T) {
	SetDev(true)
	defer SetDev(false)
	defer func() {
		msg, _ := recover().(string)
		if !strings.Contains(msg, "gx.SetIslands(gxislands.Bundle())") {
			t.Fatalf("panic = %q", msg)
		}
	}()
	Island("app/dash/Missing", `{}`)
}
