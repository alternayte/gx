package round6_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// usageIn is the hand-written input of the tool keys_usage.
type usageIn struct{}

func (usageIn) Pattern() string           { return "POST /keys/usage" }
func (*usageIn) Bind(*http.Request) error { return nil }
func (usageIn) GxTool() gx.ToolInfo {
	return gx.ToolInfo{Name: "keys_usage", Description: "Counts the calls of each API key.",
		Schema: `{"additionalProperties":false,"properties":{},"type":"object"}`}
}

const apiKey = "sk-live-4f9a1c"

// usage is a report whose shape the Go compiler does not see: the value of
// "calls" is a map with gx.Secret keys behind an interface.
func usage() map[string]any {
	return map[string]any{"total": 3, "calls": map[gx.Secret]int{gx.Secret(apiKey): 3}}
}

var usageReported = gx.Event[map[string]any]("usage-reported")

// TestSI_04_SecretMapKeyBehindAnInterface checks the run-time part of SI-04
// for a gx.Secret that is the key of a map behind an interface (SI-04:
// "Values of type gx.Secret cannot enter ... event details ... or tool
// results. Compile error where visible, redaction and dev panic at runtime
// otherwise"). The fix of review round 1 (D-293) walks the static type of
// the value. A map[string]any, the common form of a result with no struct,
// hides the type of its values from that walk and from the compiler, so
// this is the case that only the run time can find. encoding/json writes the
// key as its text: the secret goes to the agent and to the host page.
func TestSI_04_SecretMapKeyBehindAnInterface(t *testing.T) {
	report := gx.Action(func(c *gx.Ctx, in usageIn) error {
		c.Emit(usageReported(usage()))
		gx.ToolResult(c, usage())
		return nil
	}).Tool()
	app := gx.New(gx.Config{Adapter: noAdapter{}})
	app.Group("/", gx.Collect(report))

	t.Run("tool result", func(t *testing.T) {
		answer := app.CallTool(context.Background(), http.Header{}, "keys_usage", json.RawMessage(`{}`))
		if strings.Contains(answer.Text, apiKey) || strings.Contains(string(answer.Structured), apiKey) {
			t.Errorf("the answer of the tool for the agent holds the text of a gx.Secret: %s", answer.Text)
		}
	})
	t.Run("event detail", func(t *testing.T) {
		req := httptest.NewRequest("POST", "https://app.example/keys/usage", strings.NewReader(`{"signals":{}}`))
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Gx-Widget", "acme-keys")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		body := rec.Body.String()
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Fatalf("the fixture is wrong: status %d: %s", rec.Code, body)
		}
		if strings.Contains(body, apiKey) {
			t.Errorf("the answer of the action to the widget holds the text of a gx.Secret in the detail of an event: %s", body)
		}
	})
}
