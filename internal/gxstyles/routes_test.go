package gxstyles_test

import (
	"context"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/gxstyles"
)

// TestREQ_STY_13_GeneratedRoutes checks the generated package: Routes gives
// the stylesheet of each route by its pattern, and two routes with one
// stylesheet share one constant (REQ-STY-13).
func TestREQ_STY_13_GeneratedRoutes(t *testing.T) {
	src := string(gxstyles.GenerateAll([]byte(".app{}"), nil, map[string][]byte{
		"GET /a": []byte(".one{}"),
		"GET /b": []byte(".one{}"),
		"GET /c": []byte(".two{}"),
	}))
	for _, want := range []string{
		"func Routes() map[string][]byte {",
		`"GET /a": []byte(routeSheet0),`,
		`"GET /b": []byte(routeSheet0),`,
		`"GET /c": []byte(routeSheet1),`,
		`const routeSheet0 = ".one{}"`,
		`const routeSheet1 = ".two{}"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the generated package lacks %s:\n%s", want, src)
		}
	}
	if strings.Contains(src, "routeSheet2") {
		t.Errorf("two routes with one stylesheet have two constants:\n%s", src)
	}
	if empty := string(gxstyles.Generate(nil)); !strings.Contains(empty, "func Routes() map[string][]byte {\n\treturn map[string][]byte{}\n}") {
		t.Errorf("an app with no route stylesheet has no empty Routes:\n%s", empty)
	}
}

// TestREQ_STY_13_LimitOfClassSets checks the limit of distinct class sets:
// above it the build makes no route stylesheet and says so, and a negative
// limit turns the route stylesheets off with no message (REQ-STY-13).
func TestREQ_STY_13_LimitOfClassSets(t *testing.T) {
	lists := map[string][]string{
		"GET /a": {"one"},
		"GET /b": {"one"},
		"GET /c": {"two"},
		"GET /d": {"three"},
	}
	// Three distinct sets and a limit of two: no Tailwind run.
	sheets, notice, err := gxstyles.BuildRouteLists(context.Background(), t.TempDir(), lists, true, 2)
	if err != nil || sheets != nil {
		t.Fatalf("over the limit: sheets %v, err %v", sheets, err)
	}
	for _, want := range []string{"3 distinct class sets", "the limit is 2", "route_sheets", "app.css"} {
		if !strings.Contains(notice, want) {
			t.Errorf("the notice lacks %q: %s", want, notice)
		}
	}
	sheets, notice, err = gxstyles.BuildRouteLists(context.Background(), t.TempDir(), lists, true, -1)
	if err != nil || sheets != nil || notice != "" {
		t.Errorf("a negative limit: sheets %v, notice %q, err %v", sheets, notice, err)
	}
}
