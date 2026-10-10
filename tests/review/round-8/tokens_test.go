package round8_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/internal/gxstyles"
)

// TestREQ_STY_14_TokenWithAColourName checks REQ-STY-14: "The build writes
// the theme tokens of `app/theme.css` as Go values (hex colours, pixel
// sizes) in one generated file, for light and dark", with the acceptance "A
// changed token changes the colour of the email button."
//
// The app changes the token --primary of the default theme to a colour
// name of CSS. The parser of the tokens reads the names white and black
// only, so the generated file has no --primary. The button of the email
// kit reads var(--primary): gx.RenderEmail then returns an error, and the
// colour of the button does not change.
func TestREQ_STY_14_TokenWithAColourName(t *testing.T) {
	const was = "--primary: oklch(0.205 0 0);"
	if !strings.Contains(gx.DefaultThemeCSS, was) {
		t.Fatalf("the fixture is wrong: the default theme has no %q", was)
	}
	theme := strings.Replace(gx.DefaultThemeCSS, was, "--primary: navy;", 1)
	light, _ := gxstyles.ThemeValues([]byte(theme))
	if got := light["--primary"]; got != "#000080" {
		t.Errorf("the theme has `--primary: navy`, and the light tokens have --primary = %q, want \"#000080\"", got)
	}
	// The cell of the button of the email kit (registry/email/Button.gx).
	cell := gx.El("td", gx.Attrs{{Key: "style", Value: "background-color:var(--primary)"}}, gx.Text("Pay"))
	msg, err := gx.RenderEmail(cell, gx.EmailOptions{BaseURL: "https://shop.example", Tokens: light})
	if err != nil {
		t.Errorf("the email button with the tokens of the theme: %v", err)
	} else if !strings.Contains(msg.HTML, "background-color:#000080") {
		t.Errorf("the email button does not have the colour of the changed token: %s", msg.HTML)
	}
}
