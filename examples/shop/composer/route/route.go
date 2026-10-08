// Package route holds the route types of the composer slice (DR-01).
package route

import "github.com/alternayte/gx"

// Widget is the first render of the composer widget. Its field is the
// attribute of the <shop-composer> element on a host page.
type Widget struct {
	gx.Route `GET /widgets/composer`
	To       string `query:"to"`
}

// Send is the form of the composer.
type Send struct {
	gx.Route   `POST /widgets/composer/send`
	To         string
	Subject    string
	Body       string
	Priority   string
	Attachment gx.File
}

// Rules declares the validation rules of a message.
func (in *Send) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.To, gx.Required, gx.Email),
		gx.Field(&in.Subject, gx.Required, gx.MaxLen(80)),
		gx.Field(&in.Body, gx.Required, gx.MinLen(10)),
		gx.Field(&in.Priority, gx.OneOf("low", "normal", "high")),
		gx.Field(&in.Attachment, gx.MaxSize(64<<10), gx.Accept("text/*")),
	}
}

// Discard drops the draft of the composer.
type Discard struct {
	gx.Route `POST /widgets/composer/discard`
}
