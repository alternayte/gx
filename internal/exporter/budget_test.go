package exporter_test

import (
	"context"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/exporter"
	"github.com/alternayte/gx/internal/gxconfig"
)

// TestREQ_DEV_13_Budgets checks the budget of a page route: a page that
// loads more gzipped bytes of JS or of CSS than its budget is a finding that
// names each file; a route of the table has its own numbers; and an app with
// no budget has no check (REQ-DEV-13).
func TestREQ_DEV_13_Budgets(t *testing.T) {
	// No budget: the check does not start the app, so a directory with no
	// app is no error.
	if got, err := exporter.Budgets(context.Background(), t.TempDir(), "", gxconfig.Budget{}); err != nil || got != nil {
		t.Fatalf("no budget: findings %v, err %v", got, err)
	}

	dir := exportApp(t, nil)
	// The home page has signals, so it loads the runtime and the adapter.
	// Each page of the app loads the stylesheet.
	findings, err := exporter.Budgets(context.Background(), dir, "", gxconfig.Budget{
		JS:  2000,
		CSS: 100000,
		Routes: map[string]gxconfig.RouteBudget{
			// The track page has its own numbers: JS with no limit that
			// it can pass, and a CSS budget of ten bytes.
			"GET /track": {JS: 500000, CSS: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]exporter.BudgetFinding{}
	for _, f := range findings {
		by[f.Path+" "+f.Kind] = f
	}
	home, ok := by["/ JS"]
	if !ok {
		t.Fatalf("findings = %v, want the JS of the home page", findings)
	}
	if home.Limit != 2000 || home.Bytes <= 2000 || len(home.Files) < 2 {
		t.Errorf("the home page: %+v, want more than 2000 bytes in two files or more", home)
	}
	sum, names := 0, ""
	for i, f := range home.Files {
		sum += f.Bytes
		names += f.URL + " "
		if i > 0 && f.Bytes > home.Files[i-1].Bytes {
			t.Errorf("the files are not in order of size: %v", home.Files)
		}
	}
	if sum != home.Bytes || !strings.Contains(names, "/_gx/gx.") || !strings.Contains(names, "datastar") {
		t.Errorf("the files %q sum to %d, want the runtime and the adapter with the sum %d", names, sum, home.Bytes)
	}
	if msg := home.String(); !strings.Contains(msg, "GET /") || !strings.Contains(msg, "its budget is 2000") || !strings.Contains(msg, home.Files[0].URL) {
		t.Errorf("the message does not name the route, the budget and the files: %s", msg)
	}
	// The route of the table: its JS budget holds, and its CSS budget
	// does not.
	if _, over := by["/track JS"]; over {
		t.Errorf("the track page is over its own JS budget: %+v", by["/track JS"])
	}
	track, ok := by["/track CSS"]
	if !ok || track.Pattern != "GET /track" || track.Limit != 10 || len(track.Files) != 1 {
		t.Errorf("the CSS of the track page: %+v, want one stylesheet over the budget of 10", track)
	}
	// The CSS of the home page is inside the default.
	if _, over := by["/ CSS"]; over {
		t.Errorf("the home page is over the default CSS budget: %+v", by["/ CSS"])
	}
}
