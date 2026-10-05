package toast

import "github.com/alternayte/gx"

// fixtureWidth is the width a toast has inside the toaster.
const fixtureWidth = "max-w-[356px]"

var ToastFixtures = gx.Fixtures[ToastProps]{
	"Default": {Class: fixtureWidth, Toast: gx.ToastPatch{Text: "Event created"}},
	"Success": {Class: fixtureWidth, Toast: gx.ToastPatch{Kind: gx.ToastSuccess, Text: "Changes saved"}},
	"Info":    {Class: fixtureWidth, Toast: gx.ToastPatch{Kind: gx.ToastInfo, Text: "A new version is available"}},
	"Warning": {Class: fixtureWidth, Toast: gx.ToastPatch{Kind: gx.ToastWarning, Text: "Your trial ends in 3 days"}},
	"Error":   {Class: fixtureWidth, Toast: gx.ToastPatch{Kind: gx.ToastError, Text: "The upload failed"}},
	"Loading": {Class: fixtureWidth, Toast: gx.ToastPatch{Kind: gx.ToastLoading, Text: "Uploading the file"}},
	"WithDescription": {Class: fixtureWidth, Toast: gx.ToastPatch{
		Kind:        gx.ToastSuccess,
		Text:        "Event created",
		Description: "Monday, 12 January at 09:00",
	}},
	"WithAction": {Class: fixtureWidth, Toast: gx.ToastPatch{
		Text:        "Item added to the cart",
		Description: "Open the cart to check out.",
		Action:      gx.ToastAction{Label: "View", URL: "/"},
	}},
}
