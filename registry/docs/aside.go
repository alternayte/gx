package docs

import "github.com/alternayte/gx"

// kindClass maps a callout tone to its accent classes (REQ-STY-05).
var kindClass = gx.Enum[Kind]{
	Note:    "",
	Tip:     "gx-aside-tip",
	Caution: "gx-aside-caution",
	Danger:  "gx-aside-danger",
}

// Kind is the tone of a docs callout (REQ-CNT-05).
type Kind string

// The tones of docs.Aside.
const (
	Note    Kind = "note"
	Tip     Kind = "tip"
	Caution Kind = "caution"
	Danger  Kind = "danger"
)
