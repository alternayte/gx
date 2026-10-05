// Package datastar adapts Gx to the Datastar hypermedia library through the
// public gx.Adapter hook (REQ-PLG-04, DR-04). It imports only the public Gx
// API, the standard library and the official Datastar Go SDK.
package datastar

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alternayte/gx"
	sdk "github.com/starfederation/datastar-go/datastar"
)

// Version is the pinned Datastar browser runtime version (SI-10).
const Version = "v1.0.4"

// bundleSHA256 is the SHA-256 of the pinned browser runtime. A mismatch
// stops the build (SI-10).
const bundleSHA256 = "727844adfc825ee651fb93c544a2a739986f9a21820a94524b35f0cac470cf91"

//go:embed datastar.js
var bundle []byte

type adapter struct{}

// Adapter returns the Datastar adapter for gx.Config.Adapter.
func Adapter() gx.Adapter { return adapter{} }

func (adapter) Name() string  { return "datastar" }
func (adapter) Signals() bool { return true }

// Runtime returns the scripts every page needs (request lifecycle step 6).
// The Gx runtime script comes from package gx.
func (adapter) Runtime() gx.Node {
	base := gx.BasePath()
	return gx.El("script", gx.Attrs{
		{Key: "type", Value: "module"},
		{Key: "src", Value: base + "/_gx/datastar.js", Kind: gx.AttrURL},
		{Key: "data-gx-adapter", Value: "datastar"},
	})
}

// Assets returns the browser runtime files served under /_gx/.
func (adapter) Assets() map[string][]byte {
	check := sha256.Sum256(bundle)
	if got := hex.EncodeToString(check[:]); got != bundleSHA256 {
		panic("datastar: browser runtime hash mismatch: " + got)
	}
	return map[string][]byte{
		"datastar.js": bundle,
	}
}

// Respond writes the patched answer as Datastar SSE events.
func (adapter) Respond(w http.ResponseWriter, r *http.Request, res *gx.Response) error {
	if res.Status >= 400 {
		// Datastar reads the events on an error status too, so the
		// toast still arrives (REQ-ACT-10).
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		w.WriteHeader(res.Status)
	}
	sse := sdk.NewSSE(w, r)
	for _, p := range res.Patches {
		switch t := p.(type) {
		case gx.ElementPatch:
			// Target is a CSS selector: "#id" or "[data-gx-slot=...]".
			opts := []sdk.PatchElementOption{sdk.WithSelector(t.Target)}
			switch t.Mode {
			case gx.ModeInner:
				opts = append(opts, sdk.WithModeInner())
			case gx.ModeAppend:
				opts = append(opts, sdk.WithModeAppend())
			case gx.ModePrepend:
				opts = append(opts, sdk.WithModePrepend())
			case gx.ModeReplace:
				opts = append(opts, sdk.WithModeReplace())
			case gx.ModeRemove:
				opts = append(opts, sdk.WithModeRemove())
			}
			if t.Transition {
				opts = append(opts, sdk.WithViewTransitions())
			}
			if err := sse.PatchElements(gx.StringRequest(r, t.Node), opts...); err != nil {
				return err
			}
		case gx.SignalPatch:
			value := scopeObject(t.Scope, t.Signals)
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if err := sse.PatchSignals(data); err != nil {
				return err
			}
		case gx.RedirectPatch:
			if err := sse.Redirect(t.URL); err != nil {
				return err
			}
		case gx.ToastPatch:
			// Append, so the toaster region keeps its id for the next
			// toast (REQ-REG-11). A toast with an ID carries it as its
			// element id, and the behaviour runtime puts it in the place
			// of the earlier toast with that id. The pinned Datastar
			// logs a warning for every selector with no match, so the
			// adapter does not send a replace that can miss.
			if err := sse.PatchElements(gx.StringRequest(r, gx.RenderToast(r, t)), sdk.WithSelectorID("gx-toaster"), sdk.WithModeAppend()); err != nil {
				return err
			}
		}
	}
	if res.Navigate && res.Head != nil {
		// The Gx runtime reads gx-head and merges title, meta and link
		// tags (REQ-RTE-12).
		data, err := json.Marshal(res.Head)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: gx-head\ndata: %s\n\n", data); err != nil {
			return err
		}
	}
	return nil
}

// Invoke returns the data-on:click attribute that calls the Datastar action
// of the method. The scope travels in the Gx-Scope header (REQ-ACT-03).
func (adapter) Invoke(method, url, scope string) gx.Attr {
	var fn string
	switch method {
	case "GET":
		fn = "@get"
	case "POST":
		fn = "@post"
	case "PUT":
		fn = "@put"
	case "PATCH":
		fn = "@patch"
	case "DELETE":
		fn = "@delete"
	default:
		return gx.Attr{}
	}
	value := fn + "('" + url + "')"
	if scope != "" {
		value = fn + "('" + url + "', {headers: {'Gx-Scope': '" + scope + "'}})"
	}
	return gx.Attr{Key: "data-on:click", Value: value}
}

// ReadSignals decodes the request signals, adapter-native, into dst.
func (adapter) ReadSignals(r *http.Request, dst any) error {
	return sdk.ReadSignals(r, dst)
}

// scopeObject nests v under the dotted scope path (REQ-ACT-06). An empty
// scope returns v unchanged.
func scopeObject(scope string, v any) any {
	if scope == "" {
		return v
	}
	parts := strings.Split(scope, ".")
	var out any = v
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "" {
			continue
		}
		out = map[string]any{parts[i]: out}
	}
	return out
}

var _ gx.Adapter = adapter{}
