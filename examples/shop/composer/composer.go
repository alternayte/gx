// Package composer is the message composer of the reference app. It is a
// widget: a page of a different site shows it with the <shop-composer>
// element, and the shop renders it and answers its form (REQ-ISL-20).
package composer

import (
	"strings"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/composer/route"
)

// SentDetail is the detail of the composer-sent event.
type SentDetail struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	// Attachment is the name of the file of the message, and Bytes is its
	// size. A message with no file has an empty name.
	Attachment string `json:"attachment"`
	Bytes      int64  `json:"bytes"`
}

// NoteCountProps are the props of the NoteCount island.
type NoteCountProps struct {
	// Note is the signal of the composer that holds the note.
	Note gx.SignalRef[string] `json:"note"`
	// Limit is the number of characters that a note can have.
	Limit int `json:"limit"`
}

// Sent tells the host page that the shop took a message.
var Sent = gx.Event[SentDetail]("composer-sent")

// Send is the form of the composer. The rules of route.Send run first.
var Send = gx.Form(func(c *gx.Ctx, in *route.Send) error {
	if strings.HasSuffix(in.To, "@blocked.example") {
		return gx.FieldError(&in.To, "composer.blocked")
	}
	c.Emit(Sent(SentDetail{To: in.To, Subject: in.Subject, Attachment: in.Attachment.Name, Bytes: in.Attachment.Size}))
	return c.Toast("Message sent", gx.ToastSuccess)
}, Composer)

// Discard answers the button of the discard dialog.
var Discard = gx.Action(func(c *gx.Ctx, in route.Discard) error {
	return c.Toast("Draft discarded")
})

// Widget is the composer as a widget. The attribute "to" fills the first
// field.
var Widget = gx.Widget(func(c *gx.Ctx, in route.Widget) (ComposerProps, error) {
	return Send.Props(&route.Send{To: in.To, Priority: "normal"}), nil
}, Composer).Tag("shop-composer")

// Routes collects the composer widget, its form and its action. The app
// mounts them in a group with gx.AllowOrigins.
var Routes = gx.Collect(Widget, Send, Discard)
