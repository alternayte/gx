package email

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/gxstyles"
)

var update = flag.Bool("update", false, "write the golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs from the golden file:\n%s", name, got)
	}
}

// TestREQ_REG_16_OrderEmail covers gx.RenderEmail on the order email of the
// shop, which is generated code with the email kit: the golden HTML, the
// golden text, and each link as an absolute URL from the base URL. The
// output has no class, no CSS variable and no oklch colour (REQ-REG-15,
// REQ-STY-14).
func TestREQ_REG_16_OrderEmail(t *testing.T) {
	msg, err := gx.RenderEmail(Order(OrderFixtures["TwoItems"]), gx.EmailOptions{
		BaseURL: "https://shop.example",
		Tokens:  gxstyles.LightTokens(),
		Title:   "Your order 1042",
	})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "order.html", msg.HTML)
	golden(t, "order.txt", msg.Text)

	links := regexp.MustCompile(`(?:href|src)="([^"]*)"`).FindAllStringSubmatch(msg.HTML, -1)
	if len(links) != 3 {
		t.Errorf("the email has %d addresses, want the image, the button and the link", len(links))
	}
	for _, m := range links {
		if !strings.HasPrefix(m[1], "https://shop.example/") {
			t.Errorf("the address %q does not start with the base URL", m[1])
		}
	}
	if !strings.Contains(msg.HTML, `href="https://shop.example/basket"`) {
		t.Error("the typed link of the button is not the absolute URL of the basket page")
	}
	for _, bad := range []string{" class=", "var(", "oklch", "<script", "<style", "data-gx-", "data-signals", "data-on"} {
		if strings.Contains(msg.HTML, bad) {
			t.Errorf("the HTML of the email has %q", bad)
		}
	}
	if !strings.HasPrefix(msg.HTML, "<!DOCTYPE html><html lang=\"en\">") || !strings.HasSuffix(msg.HTML, "</body></html>") {
		t.Error("the HTML is not a full document")
	}
	for _, want := range []string{"Your order is on its way", "2 x Green tea CHF 24.00", "1 x Cup & saucer CHF 5.50", "Total CHF 29.50", "See your basket (https://shop.example/basket)", "Gx shop"} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("the text lacks %q:\n%s", want, msg.Text)
		}
	}

	// The dark tokens give the dark colours of the same email.
	dark, err := gx.RenderEmail(Order(OrderFixtures["TwoItems"]), gx.EmailOptions{BaseURL: "https://shop.example", Tokens: gxstyles.DarkTokens()})
	if err != nil {
		t.Fatal(err)
	}
	if dark.HTML == msg.HTML || dark.Text != msg.Text {
		t.Error("the dark tokens do not change only the colours")
	}
}
