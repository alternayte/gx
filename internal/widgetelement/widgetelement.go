// Package widgetelement writes the element file of a widget (REQ-ISL-19): the
// small loader that a host page loads. `gx wc build`, the dev host page and
// the app, which serves it at /_gx/widgets/<tag>.js, use it.
package widgetelement

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/alternayte/gx/internal/elementname"
)

// loader is the built element file with a placeholder for its
// configuration. `just runtime` rebuilds it from runtime/js/widget-element.ts.
//
//go:embed element.js
var loader []byte

const placeholder = "__GX_WIDGET_CONFIG__"

// Config is the configuration of one element file.
type Config struct {
	// Tag is the element name, for example "acme-cart".
	Tag string `json:"tag"`
	// Attrs are the attributes that the element sends to the server.
	Attrs []string `json:"attrs"`
	// Server is the origin of the Gx server, for example
	// "https://api.acme.dev". Empty means the origin of the host page.
	Server string `json:"server"`
	// Path is the path of the GET route of the widget.
	Path string `json:"path"`
	// Self marks a file that the Gx server serves: the origin of the
	// server is then the origin of the URL of the file.
	Self bool `json:"self,omitempty"`
}

// File returns the element file of one widget.
func File(c Config) ([]byte, error) {
	if problem := elementname.Problem(c.Tag); problem != "" {
		return nil, errors.New("widget tag " + c.Tag + ": " + problem)
	}
	if !strings.HasPrefix(c.Path, "/") {
		return nil, errors.New("the path of the widget route must start with /")
	}
	if c.Server != "" {
		u, err := url.Parse(c.Server)
		if err != nil || u.Scheme == "" || u.Host == "" || strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.New("the server of a widget is an origin, for example https://api.acme.dev")
		}
		c.Server = u.Scheme + "://" + u.Host
	}
	if c.Attrs == nil {
		c.Attrs = []string{}
	}
	// encoding/json escapes <, > and & and the line separators, so the
	// text is safe inside a script.
	cfg, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	if bytes.Count(loader, []byte(placeholder)) != 1 {
		return nil, errors.New("the built element file has no configuration placeholder; run just runtime")
	}
	return bytes.Replace(loader, []byte(placeholder), cfg, 1), nil
}
