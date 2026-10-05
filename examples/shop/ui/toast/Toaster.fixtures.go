package toast

import "github.com/alternayte/gx"

// The fixture toaster is static, so it stays inside its gallery section, and
// its toast is sticky, so it stays for the audits.
var ToasterFixtures = gx.Fixtures[ToasterProps]{
	"WithToast": {Class: "static", Children: Render(gx.ToastPatch{Text: "Saved", Sticky: true})},
}
