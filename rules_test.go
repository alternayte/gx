package gx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

type tamperRoute struct{ Qty int }

func (tamperRoute) Pattern() string { return "POST /tamper" }

func (in *tamperRoute) Bind(r *http.Request) error {
	var m map[string]int
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return err
	}
	in.Qty = m["qty"]
	return nil
}

func (in *tamperRoute) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Qty, gx.Min(1), gx.Max(5))}
}

// TestSI_13_TamperedSignal checks a signal-bound field that fails its rules
// answers a field error (SI-13).
func TestSI_13_TamperedSignal(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in tamperRoute) error { return nil })
	app := gx.New(gx.Config{Adapter: a})
	app.Group("/", gx.Collect(h))

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/tamper", strings.NewReader(body))
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec
	}
	if rec := post(`{"qty":99}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "at most 5") {
		t.Fatalf("tampered = %d %q, want 422 with the max message", rec.Code, rec.Body.String())
	}
	if rec := post(`{"qty":3}`); rec.Code != http.StatusNoContent {
		t.Fatalf("valid = %d, want 204", rec.Code)
	}
}
