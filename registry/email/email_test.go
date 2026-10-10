package email_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/internal/gxstyles"
	"github.com/alternayte/gx/registry/email"
)

// each returns the node of each fixture of each part of the kit, by
// "<Part>/<fixture>".
func each() map[string]gx.Node {
	out := map[string]gx.Node{}
	add := func(part, name string, n gx.Node) { out[part+"/"+name] = n }
	for name, p := range email.DocumentFixtures {
		add("Document", name, email.Document(p))
	}
	for name, p := range email.SectionFixtures {
		add("Section", name, email.Section(p))
	}
	for name, p := range email.RowFixtures {
		add("Row", name, email.Row(p))
	}
	for name, p := range email.ColumnFixtures {
		add("Column", name, email.ColumnWrap(email.Column(p)))
	}
	for name, p := range email.TextFixtures {
		add("Text", name, email.Text(p))
	}
	for name, p := range email.HeadingFixtures {
		add("Heading", name, email.Heading(p))
	}
	for name, p := range email.ButtonFixtures {
		add("Button", name, email.Button(p))
	}
	for name, p := range email.ImageFixtures {
		add("Image", name, email.Image(p))
	}
	for name, p := range email.DividerFixtures {
		add("Divider", name, email.Divider(p))
	}
	return out
}

// TestREQ_REG_15_EmailKit covers the kit: the nine parts are in the item,
// each has a fixture, the output of each fixture has no class attribute
// and no style element, and gx.RenderEmail takes each one with the tokens
// of the default theme.
func TestREQ_REG_15_EmailKit(t *testing.T) {
	data, err := os.ReadFile("gx-item.json")
	if err != nil {
		t.Fatal(err)
	}
	var item struct {
		Name  string
		Files []struct{ Path string }
	}
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, f := range item.Files {
		files[f.Path] = true
	}
	if _, err := os.Stat("USAGE.md"); err != nil || item.Name != "email" {
		t.Errorf("the item email has no USAGE.md, or a different name: %v", err)
	}
	fixtures := each()
	parts := []string{"Document", "Section", "Row", "Column", "Text", "Heading", "Button", "Image", "Divider"}
	for _, part := range parts {
		if !files[part+".gx"] || !files[part+".fixtures.go"] {
			t.Errorf("the item does not list %s.gx and %s.fixtures.go", part, part)
		}
		found := false
		for name := range fixtures {
			found = found || strings.HasPrefix(name, part+"/")
		}
		if !found {
			t.Errorf("the part %s has no fixture", part)
		}
	}

	light, _ := gxstyles.ThemeValues([]byte(gx.DefaultThemeCSS))
	names := make([]string, 0, len(fixtures))
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var b bytes.Buffer
		if err := gx.RenderNode(&b, fixtures[name]); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		html := b.String()
		// The render of a page marks each typed address for the active
		// link (data-gx-active); gx.RenderEmail removes the marker.
		for _, bad := range []string{" class=", "<style", "<script", "data-signals"} {
			if strings.Contains(html, bad) {
				t.Errorf("%s: the output has %q:\n%s", name, bad, html)
			}
		}
		msg, err := gx.RenderEmail(fixtures[name], gx.EmailOptions{BaseURL: "https://shop.example", Tokens: light})
		if err != nil {
			t.Errorf("%s: gx.RenderEmail: %v", name, err)
			continue
		}
		for _, bad := range []string{"var(", "oklch", "data-gx-", " class="} {
			if strings.Contains(msg.HTML, bad) {
				t.Errorf("%s: the email has %q", name, bad)
			}
		}
	}
}

// TestREQ_STY_14_TokenChangesTheButton covers the path from the theme file
// to the email: a changed token of app/theme.css changes the colour of the
// button of the kit.
func TestREQ_STY_14_TokenChangesTheButton(t *testing.T) {
	render := func(theme string) string {
		t.Helper()
		light, _ := gxstyles.ThemeValues([]byte(theme))
		msg, err := gx.RenderEmail(email.Button(email.ButtonFixtures["Default"]), gx.EmailOptions{BaseURL: "https://shop.example", Tokens: light})
		if err != nil {
			t.Fatal(err)
		}
		return msg.HTML
	}
	before := render(gx.DefaultThemeCSS)
	if !strings.Contains(before, "background-color:#171717;border-radius:10px") || !strings.Contains(before, `href="https://shop.example/orders/1042"`) {
		t.Fatalf("the button of the default theme:\n%s", before)
	}
	changed := strings.Replace(gx.DefaultThemeCSS, "--primary: oklch(0.205 0 0);", "--primary: oklch(0.6 0.2 250);", 1)
	if changed == gx.DefaultThemeCSS {
		t.Fatal("the default theme has no --primary: oklch(0.205 0 0)")
	}
	after := render(changed)
	if strings.Contains(after, "#171717") || !strings.Contains(after, "background-color:#") || after == before {
		t.Errorf("the changed token did not change the colour of the button:\n%s", after)
	}
}

// TestREQ_REG_16_ShopOrderEmail runs the golden test of the order email of
// the shop. The shop is a module of its own, so `go test ./...` of the repo
// does not reach it.
func TestREQ_REG_16_ShopOrderEmail(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-run", "TestREQ_REG_16_OrderEmail", "-v", "./basket/email")
	cmd.Dir = filepath.Join("..", "..", "examples", "shop")
	out, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("--- PASS: TestREQ_REG_16_OrderEmail")) {
		t.Fatalf("the order email test of the shop: %v\n%s", err, out)
	}
}
