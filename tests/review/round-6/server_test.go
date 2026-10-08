// Package round6_test holds the blocking findings of review round 6 for
// release 0.3.0 (SDD §16.3). Every test fails on the reviewed HEAD 316debc.
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

// noAdapter is an adapter that answers with a status only.
type noAdapter struct{}

func (noAdapter) Name() string              { return "none" }
func (noAdapter) Signals() bool             { return true }
func (noAdapter) Runtime() gx.Node          { return nil }
func (noAdapter) Assets() map[string][]byte { return nil }
func (noAdapter) Respond(w http.ResponseWriter, _ *http.Request, res *gx.Response) error {
	status := res.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	return nil
}
func (noAdapter) ReadSignals(*http.Request, any) error { return nil }
func (noAdapter) Invoke(method, url, scope string) gx.Attr {
	return gx.Attr{Key: "data-x-on", Value: method + " " + url}
}
func (noAdapter) On(inv gx.Invocation) []gx.Attr {
	return []gx.Attr{{Key: "data-x-on-" + inv.Event, Value: inv.Method + " " + inv.URL}}
}

// renameIn is the hand-written input of the tool items_rename. A generated
// route type has the same methods.
type renameIn struct {
	ID   string
	Name string
}

func (renameIn) Pattern() string { return "POST /items/{id}" }

func (in *renameIn) Bind(r *http.Request) error {
	in.ID = r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		return err
	}
	in.Name = r.PostForm.Get("name")
	return nil
}

func (renameIn) GxTool() gx.ToolInfo {
	return gx.ToolInfo{Name: "items_rename", Description: "Gives an item a new name.",
		Schema: `{"additionalProperties":false,"properties":{"id":{"type":"string"},"name":{"type":"string"}},"required":["id"],"type":"object"}`,
		Fields: []gx.ToolField{{Name: "id", In: "path"}, {Name: "name", In: "form"}}}
}

// purgeIn is the input of an action that is not a tool.
type purgeIn struct{}

func (purgeIn) Pattern() string           { return "POST /items/purge" }
func (*purgeIn) Bind(*http.Request) error { return nil }

// itemsApp returns an app with the tool items_rename and with the action
// purge, which has no .Tool(). ran names each handler that ran.
func itemsApp(ran *[]string) *gx.App {
	rename := gx.Action(func(c *gx.Ctx, in renameIn) error {
		*ran = append(*ran, "rename "+in.ID)
		return nil
	}).Tool()
	purge := gx.Action(func(c *gx.Ctx, in purgeIn) error {
		*ran = append(*ran, "purge")
		return nil
	})
	app := gx.New(gx.Config{Adapter: noAdapter{}})
	app.Group("/", gx.Collect(rename, purge))
	return app
}

// TestSI_07_ToolCallRunsOnlyTheActionOfTheTool checks that a tool call runs
// the action of the tool and no other action (SI-07: "Only actions marked
// .Tool() are tools"). The request of a tool call goes to the routes of the
// app by its path, and a path argument is a value of the agent. With the
// value "purge" for {id} of POST /items/{id}, the path is /items/purge, and
// the routes of the app give the request to the action of POST /items/purge,
// which is not a tool. The agent then runs an action that the app did not
// mark, and the cross-origin check of that action does not run a second
// time.
func TestSI_07_ToolCallRunsOnlyTheActionOfTheTool(t *testing.T) {
	t.Run("server", func(t *testing.T) {
		var ran []string
		app := itemsApp(&ran)
		if names := app.Tools(); len(names) != 1 || names[0].Name != "items_rename" {
			t.Fatalf("the fixture is wrong: the tools of the app are %v", names)
		}
		app.CallTool(context.Background(), http.Header{}, "items_rename", json.RawMessage(`{"id":"7","name":"x"}`))
		if len(ran) != 1 || ran[0] != "rename 7" {
			t.Fatalf("the fixture is wrong: the tool call with id 7 ran %v", ran)
		}
		ran = nil
		answer := app.CallTool(context.Background(), http.Header{}, "items_rename", json.RawMessage(`{"id":"purge","name":"x"}`))
		for _, name := range ran {
			if name == "purge" {
				t.Errorf("the tool call items_rename with id \"purge\" ran the action of POST /items/purge, which has no .Tool(); handlers that ran: %v; the answer for the agent: %q", ran, answer.Text)
			}
		}
	})
	t.Run("page", func(t *testing.T) {
		var ran []string
		app := itemsApp(&ran)
		req := httptest.NewRequest("POST", "https://app.example/_gx/tools/items_rename", strings.NewReader(`{"id":"purge","name":"x"}`))
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Content-Type", "application/json")
		app.ServeHTTP(httptest.NewRecorder(), req)
		for _, name := range ran {
			if name == "purge" {
				t.Errorf("the page tool call items_rename with id \"purge\" ran the action of POST /items/purge, which has no .Tool(); handlers that ran: %v", ran)
			}
		}
	})
}
