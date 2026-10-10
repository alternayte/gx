package gx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

type nonceKey struct{}

// WithNonce returns r with the CSP nonce of the response in scope. Every
// script element Gx renders for r then carries it (SI-11). gx.CSP calls it;
// an app with its own policy middleware calls it with its own nonce.
func WithNonce(r *http.Request, nonce string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), nonceKey{}, nonce))
}

// Nonce returns the CSP nonce of the request, or "" when no policy set one.
// A page author needs it only for a script inside trusted raw HTML.
func Nonce(r *http.Request) string {
	if r == nil {
		return ""
	}
	nonce, _ := r.Context().Value(nonceKey{}).(string)
	return nonce
}

// CSPOptions configure gx.CSP.
type CSPOptions struct {
	// UnsafeEval adds 'unsafe-eval' to script-src. The Datastar adapter
	// needs it: Datastar evaluates client expressions at runtime.
	UnsafeEval bool
	// Directives holds more directives, for example
	// "img-src 'self' data:; frame-ancestors 'none'".
	Directives string
}

// CSP is middleware that sets a strict Content-Security-Policy with one
// fresh nonce per response (SI-11). It works around a whole app, in an
// app.Group call and around any handler that calls gx.Render.
func CSP(opt CSPOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce := newNonce()
			policy := "script-src 'nonce-" + nonce + "' 'strict-dynamic'"
			if opt.UnsafeEval {
				policy += " 'unsafe-eval'"
			}
			policy += "; object-src 'none'; base-uri 'self'"
			if opt.Directives != "" {
				policy += "; " + opt.Directives
			}
			w.Header().Set("Content-Security-Policy", policy)
			next.ServeHTTP(w, WithNonce(r, nonce))
		})
	}
}

// newNonce returns 128 random bits in base64.
func newNonce() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// crypto/rand does not fail on a supported platform; a policy
		// with a guessable nonce must never ship.
		panic("gx: no random source for the CSP nonce: " + err.Error())
	}
	return base64.RawStdEncoding.EncodeToString(raw[:])
}

// stringNonce returns the HTML of a node Gx itself writes into the head,
// with the nonce on its scripts.
func stringNonce(n Node, nonce string) string {
	if nonce == "" {
		return String(n)
	}
	b := getBuffer()
	defer putBuffer(b)
	renderNode(b, n, &renderState{nonce: nonce})
	return b.String()
}
