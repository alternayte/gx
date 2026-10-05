package site

import (
	_ "embed"

	"github.com/alternayte/gx"
)

// The scripts of the docs app. They are docs-only: package gx has no client
// toast call and no frame sizing.
var (
	//go:embed preview.js
	previewJS string
	//go:embed toast.js
	toastJS string
)

// previewScript sets the height of the frame around a preview.
func previewScript() gx.Node { return inlineScript(previewJS) }

// toastScript copies the toast of a toast preview into its toaster.
func toastScript() gx.Node { return inlineScript(toastJS) }

// inlineScript renders trusted script text of this package. A .gx file does
// not render the body of a script element.
func inlineScript(js string) gx.Node {
	return gx.El("script", nil, gx.Raw(gx.SafeHTML(js))) //gx:trusted the script is a file of this package
}
