package lsp_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// featureModule is the module the REQ-DEV-08 features run against.
func featureModule(t *testing.T) (dir string, card string) {
	t.Helper()
	dir = module(t, map[string]string{
		"ui/badge/Badge.gx": "package badge\n\nprops {\n  // Text is the label of the badge.\n  Text string\n}\n\n<span>{p.Text}</span>\n",
		"ui/card/Card.gx":   "package card\n\nimport \"app/ui/badge\"\n\nprops {\n  // Title is the heading of the card.\n  Title string\n  Count int\n}\nsignals { Qty int = 1 }\n\n<article class=\"card\">\n  total := 5\n  <badge.Badge text={p.Title} />\n  <span #total>{total}</span>\n  <input bind:value={$Qty} />\n  <button on:click={Reset{}}>Reset</button>\n</article>\n",
		"ui/card/routes.go": "package card\n\nimport \"github.com/alternayte/gx\"\n\ntype Show struct {\n\tgx.Route `GET /card/{id}`\n\tID int\n}\n\ntype Reset struct {\n\tgx.Route `POST /card/reset`\n}\n\nvar reset = gx.Action(func(c *gx.Ctx, in Reset) error { return nil })\n\nvar showPage = gx.Page(func(c *gx.Ctx, in Show) (CardProps, error) {\n\treturn CardProps{Title: \"x\"}, nil\n}, Card)\n\nvar Routes = gx.Collect(showPage, reset)\n",
	})
	return dir, filepath.Join(dir, "ui/card/Card.gx")
}

// completionParams asks for completion at the offset of needle plus delta
// bytes.
func completionParams(path, text, needle string, delta int) map[string]any {
	line, col := position(text, needle)
	return map[string]any{
		"textDocument": map[string]any{"uri": fileURI(path)},
		"position":     map[string]any{"line": line, "character": col + delta},
	}
}

type completionList struct {
	Items []struct {
		Label  string `json:"label"`
		Kind   int    `json:"kind"`
		Detail string `json:"detail"`
	} `json:"items"`
}

func labels(list completionList) []string {
	out := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		out = append(out, item.Label)
	}
	return out
}

// TestREQ_DEV_08_LSPCompletion covers completion: component tags, props,
// attributes, signals, routes in href and actions in on: (REQ-DEV-08).
func TestREQ_DEV_08_LSPCompletion(t *testing.T) {
	_, card := featureModule(t)
	text := readBody(t, card)
	c := newClient(t, filepath.Dir(filepath.Dir(filepath.Dir(card))))
	c.initialize()
	c.didOpen(card, text)
	c.waitDiagnostics(card)

	complete := func(params map[string]any) completionList {
		t.Helper()
		msg := c.request("textDocument/completion", params)
		var list completionList
		if err := json.Unmarshal(msg.Result, &list); err != nil {
			t.Fatal(err)
		}
		return list
	}

	// A component tag: badge and the local Card component.
	got := labels(complete(completionParams(card, text, "<article", 1)))
	if !contains(got, "badge.Badge") && !contains(got, "Badge") {
		t.Fatalf("tag completions = %v", got)
	}
	if !contains(got, "div") {
		t.Fatalf("tag completions lack html tags: %v", got)
	}

	// A prop on a component tag: text on <badge.Badge ...>.
	got = labels(complete(completionParams(card, text, "<badge.Badge ", 14)))
	if !contains(got, "text") {
		t.Fatalf("prop completions = %v", got)
	}

	// A directive on an HTML element.
	got = labels(complete(completionParams(card, text, "<article ", 9)))
	if !contains(got, "show") || !contains(got, "class:") {
		t.Fatalf("directive completions = %v", got)
	}

	// Signals after $.
	got = labels(complete(completionParams(card, text, "{$Qty}", 1)))
	if !contains(got, "$Qty") {
		t.Fatalf("signal completions = %v", got)
	}

	// Routes in href. The buffer under test has no href; add one first.
	text2 := strings.Replace(text, `class="card"`, `class="card" href={Show{}}`, 1)
	c.didChange(card, text2)
	c.waitDiagnostics(card)
	got = labels(complete(completionParams(card, text2, "href={", 6)))
	if !contains(got, "Show{}") && !contains(got, "card.Show{}") {
		t.Fatalf("route completions = %v", got)
	}

	// Actions in on:.
	got = labels(complete(completionParams(card, text2, "on:click={", 10)))
	if !contains(got, "Reset{}") && !contains(got, "card.Reset{}") {
		t.Fatalf("action completions = %v", got)
	}
}

type hoverResult struct {
	Contents struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	} `json:"contents"`
}

// TestREQ_DEV_08_LSPHover covers hover with types and component signatures
// (REQ-DEV-08).
func TestREQ_DEV_08_LSPHover(t *testing.T) {
	_, card := featureModule(t)
	text := readBody(t, card)
	c := newClient(t, filepath.Dir(filepath.Dir(filepath.Dir(card))))
	c.initialize()
	c.didOpen(card, text)
	c.waitDiagnostics(card)

	line, col := position(text, "p.Title")
	msg := c.request("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(card)},
		"position":     map[string]any{"line": line, "character": col + 2},
	})
	var h hoverResult
	if err := json.Unmarshal(msg.Result, &h); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Contents.Value, "string") {
		t.Fatalf("hover on p.Title = %q", h.Contents.Value)
	}
	if !strings.Contains(h.Contents.Value, "Title is the heading of the card.") {
		t.Fatalf("hover on p.Title lacks the prop description: %q", h.Contents.Value)
	}

	line, col = position(text, "badge.Badge")
	msg = c.request("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(card)},
		"position":     map[string]any{"line": line, "character": col + 6},
	})
	if err := json.Unmarshal(msg.Result, &h); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Contents.Value, "Badge") || !strings.Contains(h.Contents.Value, "Text string") {
		t.Fatalf("hover on a component = %q", h.Contents.Value)
	}
	if !strings.Contains(h.Contents.Value, "// Text is the label of the badge.") {
		t.Fatalf("hover on a component lacks the prop description: %q", h.Contents.Value)
	}
}

type locations []struct {
	URI   string `json:"uri"`
	Range struct {
		Start struct{ Line, Character int } `json:"start"`
	} `json:"range"`
}

// TestREQ_DEV_08_LSPDefinition covers go-to-definition across .gx and .go
// (REQ-DEV-08).
func TestREQ_DEV_08_LSPDefinition(t *testing.T) {
	dir, card := featureModule(t)
	text := readBody(t, card)
	c := newClient(t, dir)
	c.initialize()
	c.didOpen(card, text)
	c.waitDiagnostics(card)

	define := func(needle string, delta int) locations {
		t.Helper()
		line, col := position(text, needle)
		msg := c.request("textDocument/definition", map[string]any{
			"textDocument": map[string]any{"uri": fileURI(card)},
			"position":     map[string]any{"line": line, "character": col + delta},
		})
		var locs locations
		if err := json.Unmarshal(msg.Result, &locs); err != nil {
			t.Fatal(err)
		}
		return locs
	}

	// A component tag points at the component file.
	if locs := define("<badge.Badge", 7); len(locs) == 0 || !strings.HasSuffix(locs[0].URI, "/ui/badge/Badge.gx") {
		t.Fatalf("component definition = %+v", locs)
	}

	// An import qualifier points at the .gx import or the package: at
	// least it must stay inside the module.
	if locs := define("badge.Badge", 1); len(locs) == 0 {
		t.Fatalf("no definition for badge.Badge")
	}

	// A route type used in href points at its Go declaration.
	text2 := strings.Replace(text, `class="card"`, `class="card" href={Show{}}`, 1)
	c.didChange(card, text2)
	c.waitDiagnostics(card)
	line, col := position(text2, "Show{}")
	msg := c.request("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(card)},
		"position":     map[string]any{"line": line, "character": col + 1},
	})
	var locs locations
	if err := json.Unmarshal(msg.Result, &locs); err != nil {
		t.Fatal(err)
	}
	if len(locs) == 0 || !strings.HasSuffix(locs[0].URI, "/ui/card/routes.go") {
		t.Fatalf("route definition = %+v", locs)
	}
}

type workspaceEdit struct {
	Changes map[string][]struct {
		Range struct {
			Start struct{ Line, Character int } `json:"start"`
			End   struct{ Line, Character int } `json:"end"`
		} `json:"range"`
		NewText string `json:"newText"`
	} `json:"changes"`
}

// TestREQ_DEV_08_LSPRename covers rename of props, fragments and signals
// (REQ-DEV-08).
func TestREQ_DEV_08_LSPRename(t *testing.T) {
	_, card := featureModule(t)
	text := readBody(t, card)
	c := newClient(t, filepath.Dir(filepath.Dir(filepath.Dir(card))))
	c.initialize()
	c.didOpen(card, text)
	c.waitDiagnostics(card)

	rename := func(needle string, delta int, newName string) workspaceEdit {
		t.Helper()
		line, col := position(text, needle)
		msg := c.request("textDocument/rename", map[string]any{
			"textDocument": map[string]any{"uri": fileURI(card)},
			"position":     map[string]any{"line": line, "character": col + delta},
			"newName":      newName,
		})
		var edit workspaceEdit
		if err := json.Unmarshal(msg.Result, &edit); err != nil {
			t.Fatal(err)
		}
		return edit
	}

	// A signal: $Qty becomes $Count in the declaration and the bind: use.
	edit := rename("$Qty", 2, "Count")
	edits := edit.Changes[fileURI(card)]
	if len(edits) < 2 {
		t.Fatalf("signal rename edits = %+v", edit.Changes)
	}
	refs := 0
	for _, e := range edits {
		if e.NewText != "$Count" && e.NewText != "Count" {
			t.Fatalf("signal rename newText = %q", e.NewText)
		}
		if e.NewText == "$Count" {
			refs++
		}
	}
	if refs == 0 {
		t.Fatalf("signal rename did not touch the $Qty references: %+v", edits)
	}

	// A fragment: #total and p.Count uses... rename the fragment name.
	edit = rename("#total", 3, "sum")
	edits = edit.Changes[fileURI(card)]
	if len(edits) == 0 {
		t.Fatalf("fragment rename edits = %+v", edit.Changes)
	}
	foundTotal := false
	for _, e := range edits {
		if e.NewText == "#sum" {
			foundTotal = true
		}
	}
	if !foundTotal {
		t.Fatalf("fragment rename edits = %+v", edits)
	}

	// A prop: p.Title becomes p.Name in this file.
	edit = rename("p.Title", 3, "Name")
	edits = edit.Changes[fileURI(card)]
	if len(edits) == 0 {
		t.Fatalf("prop rename edits = %+v", edit.Changes)
	}
	hasField := false
	for _, e := range edits {
		if e.NewText == "Name" && e.Range.Start.Line == positionLine(text, "Title string") {
			hasField = true
		}
	}
	if !hasField {
		t.Fatalf("prop rename did not touch the props field: %+v", edits)
	}
}

func positionLine(text, needle string) int {
	line, _ := position(text, needle)
	return line
}

// TestREQ_DEV_08_LSPFormatting covers document formatting through the LSP
// (REQ-DEV-08).
func TestREQ_DEV_08_LSPFormatting(t *testing.T) {
	_, card := featureModule(t)
	c := newClient(t, filepath.Dir(filepath.Dir(filepath.Dir(card))))
	c.initialize()
	unformatted := "package card\n\nprops {\n  Title    string\n}\n\n<article>{p.Title}</article>\n\n\n"
	c.didOpen(card, unformatted)
	c.waitDiagnostics(card)

	msg := c.request("textDocument/formatting", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(card)},
		"options":      map[string]any{"tabSize": 2, "insertSpaces": true},
	})
	var edits []struct {
		NewText string `json:"newText"`
	}
	if err := json.Unmarshal(msg.Result, &edits); err != nil {
		t.Fatal(err)
	}
	if len(edits) != 1 {
		t.Fatalf("format edits = %+v", edits)
	}
	if !strings.Contains(edits[0].NewText, "Title string") || strings.HasSuffix(edits[0].NewText, "\n\n\n") {
		t.Fatalf("formatted text = %q", edits[0].NewText)
	}
}

type codeActionList []struct {
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Edit  *struct {
		Changes map[string][]struct {
			NewText string `json:"newText"`
		} `json:"changes"`
	} `json:"edit"`
}

// TestREQ_DEV_08_LSPCodeActions covers add fragment params, add missing prop
// and import component (REQ-DEV-08).
func TestREQ_DEV_08_LSPCodeActions(t *testing.T) {
	dir, card := featureModule(t)
	text := readBody(t, card)
	c := newClient(t, dir)
	c.initialize()
	c.didOpen(card, text)
	// The module already holds a fragment free variable: total.
	d := c.waitDiagnostics(card)

	actions := func(text string, needle string, delta int, items json.RawMessage) codeActionList {
		t.Helper()
		line, col := position(text, needle)
		msg := c.request("textDocument/codeAction", map[string]any{
			"textDocument": map[string]any{"uri": fileURI(card)},
			"range": map[string]any{
				"start": map[string]any{"line": line, "character": col + delta},
				"end":   map[string]any{"line": line, "character": col + delta},
			},
			"context": map[string]any{"diagnostics": items},
		})
		var list codeActionList
		if err := json.Unmarshal(msg.Result, &list); err != nil {
			t.Fatal(err)
		}
		return list
	}
	itemsJSON := func(d *diagnostics) json.RawMessage {
		t.Helper()
		raw, err := json.Marshal(d.Items)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}

	// Add a free variable to the #total fragment params.
	list := actions(text, "{total}", 1, itemsJSON(d))
	found := false
	for _, a := range list {
		if a.Edit == nil {
			continue
		}
		for _, edits := range a.Edit.Changes {
			for _, e := range edits {
				if strings.Contains(e.NewText, "#total(") && strings.Contains(e.NewText, "total") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("no add-fragment-params action in %+v", list)
	}

	// Add a missing required prop on <badge.Badge />.
	noProp := strings.Replace(text, `<badge.Badge text={p.Title} />`, `<badge.Badge />`, 1)
	c.didChange(card, noProp)
	d = c.waitDiagnostics(card)
	list = actions(noProp, "badge.Badge", 6, itemsJSON(d))
	found = false
	for _, a := range list {
		if strings.Contains(a.Title, "Add required prop Text") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no add-missing-prop action in %+v", list)
	}

	// Import a component that is not imported yet.
	unknown := strings.Replace(text, "import \"app/ui/badge\"\n\n", "", 1)
	unknown = strings.Replace(unknown, "<badge.Badge text={p.Title} />", "<Badge text={p.Title} />", 1)
	c.didChange(card, unknown)
	d = c.waitDiagnostics(card)
	list = actions(unknown, "<Badge", 1, itemsJSON(d))
	found = false
	for _, a := range list {
		if a.Edit == nil {
			continue
		}
		for _, edits := range a.Edit.Changes {
			for _, e := range edits {
				if strings.Contains(e.NewText, "app/ui/badge") || e.NewText == "badge.Badge" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("no import-component action in %+v", list)
	}
}
