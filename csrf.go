package gx

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	csrfCookie = "gx_csrf"
	csrfHeader = "Gx-CSRF"
	csrfField  = "gx_csrf"
)

// csrfCheckedKey marks a request that passed the check.
type csrfCheckedKey struct{}

var crossOrigin = http.NewCrossOriginProtection()

// CSRF protects non-GET requests with Go's cross-origin protection, plus a
// token for browser-shaped requests that carry no Fetch Metadata (SI-03).
// gx.App applies it to every route. An action or a form on another router
// applies the cross-origin protection to itself, so no adoption level needs
// middleware for it (REQ-RTE-19). The token needs the cookie that this
// middleware sets, so mount it around another router to protect browsers
// without Fetch Metadata.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), csrfCheckedKey{}, true))
		ensureCSRFToken(w, r)
		crossOrigin.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// A browser without Fetch Metadata sends no Sec-Fetch-*
			// header. Its request needs the token.
			if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions &&
				r.Header.Get("Sec-Fetch-Site") == "" && r.Header.Get("Sec-Fetch-Mode") == "" && browserShaped(r) &&
				!cookielessJSON(r) && !csrfTokenOK(r) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})).ServeHTTP(w, r)
	})
}

// cookielessJSON reports whether a request is a call of the JSON wire with
// no cookie. It needs no token: with no cookie, a page of a different site
// has no credential of the user to send (SI-16). The middleware of the app
// reads the Authorization header of such a call.
func cookielessJSON(r *http.Request) bool {
	return wantsJSON(r) && len(r.Header.Values("Cookie")) == 0
}

// browserShaped reports whether a request looks like a browser request: it
// names an Origin, or it carries a form body. An HTML form has one of three
// encodings, and a form of a different site sends each one with no
// preflight.
func browserShaped(r *http.Request) bool {
	if r.Header.Get("Origin") != "" || r.Header.Get("Referer") != "" {
		return true
	}
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "application/x-www-form-urlencoded") || strings.HasPrefix(ct, "multipart/form-data") ||
		strings.HasPrefix(ct, "text/plain")
}

// sameOrigin applies the cross-origin protection to a handler that no CSRF
// middleware wraps (SI-03, REQ-RTE-19).
func sameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if r.Context().Value(csrfCheckedKey{}) != nil {
		return true
	}
	if err := crossOrigin.Check(r); err != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return false
	}
	return true
}

// csrfTokenOK compares the token in the request with the cookie.
func csrfTokenOK(r *http.Request) bool {
	c, err := r.Cookie(csrfCookie)
	if err != nil || c.Value == "" {
		return false
	}
	got := r.Header.Get(csrfHeader)
	if got == "" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		_ = r.ParseForm()
		got = r.FormValue(csrfField)
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(c.Value)) == 1
}

// ensureCSRFToken sets the token cookie when the request has none.
func ensureCSRFToken(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(csrfCookie); err == nil && c.Value != "" {
		return
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    hex.EncodeToString(buf),
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}
