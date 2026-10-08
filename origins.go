package gx

import (
	"context"
	"net/http"
	"strings"
)

// AnyOrigin lists each origin in AllowOrigins. Use it for a public widget.
// AllowCredentials does not take it (SI-14).
const AnyOrigin = "*"

// originPattern is one listed origin: an exact origin, the subdomains of a
// host, or each origin.
type originPattern struct {
	any bool
	// wildcard is true for "scheme://*.host".
	wildcard bool
	scheme   string
	// host is the host and the port. For a wildcard it is the part after
	// "*.".
	host string
}

func (p originPattern) match(origin string) bool {
	if p.any {
		return true
	}
	rest, ok := strings.CutPrefix(origin, p.scheme+"://")
	if !ok {
		return false
	}
	if !p.wildcard {
		return rest == p.host
	}
	labels, ok := strings.CutSuffix(rest, "."+p.host)
	return ok && labels != "" && hostChars(labels)
}

// hostChars reports whether s holds only characters of a host name.
func hostChars(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

// originsOption is the group option of AllowOrigins and AllowCredentials.
type originsOption struct {
	patterns    []originPattern
	credentials bool
}

// AllowOrigins lists the origins that can call the routes that follow it in
// a Group from a different origin (REQ-ISL-22). An origin is exact
// ("https://shop.example.com"), the subdomains of a host
// ("https://*.partner.io"), or AnyOrigin. A request from a listed origin
// carries no cookie: the server removes the Cookie header before the
// middleware runs (SI-14). The user of such a request comes from a token.
//
// AllowOrigins panics for an origin that has no scheme, has a path, or has a
// wildcard that is not the first label of the host.
func AllowOrigins(origins ...string) originsOption {
	return originsOption{patterns: parseOrigins("AllowOrigins", origins)}
}

// AllowCredentials lists the origins that can call the routes that follow it
// in a Group from a different origin with the cookies of the user (SI-14).
// It takes exact origins only: a wildcard trusts each later subdomain
// with the session of each user. Use it for an app of your own on a
// different subdomain.
//
// AllowCredentials panics for a wildcard and for AnyOrigin.
func AllowCredentials(origins ...string) originsOption {
	patterns := parseOrigins("AllowCredentials", origins)
	for i, p := range patterns {
		if p.any || p.wildcard {
			panic("gx: AllowCredentials(" + origins[i] + "): an origin with cookies must be exact. A wildcard trusts each later subdomain with the session of the user. List each origin, or use AllowOrigins and a token.")
		}
	}
	return originsOption{patterns: patterns, credentials: true}
}

func parseOrigins(fn string, origins []string) []originPattern {
	if len(origins) == 0 {
		panic("gx: " + fn + " needs one origin or more")
	}
	out := make([]originPattern, len(origins))
	for i, o := range origins {
		p, problem := parseOrigin(o)
		if problem != "" {
			panic("gx: " + fn + "(" + o + "): " + problem + ". Write an origin as https://shop.example.com or https://*.example.com.")
		}
		out[i] = p
	}
	return out
}

// parseOrigin reads one origin of an option. It returns the problem of a
// wrong origin as text.
func parseOrigin(o string) (originPattern, string) {
	if o == AnyOrigin {
		return originPattern{any: true}, ""
	}
	o = strings.ToLower(o)
	scheme, host, ok := strings.Cut(o, "://")
	if !ok || scheme == "" {
		return originPattern{}, "the origin has no scheme"
	}
	if host == "" {
		return originPattern{}, "the origin has no host"
	}
	if strings.ContainsAny(host, "/?#@ ") {
		return originPattern{}, "an origin is a scheme, a host and a port only"
	}
	p := originPattern{scheme: scheme, host: host}
	if rest, ok := strings.CutPrefix(host, "*."); ok {
		p.wildcard, p.host = true, rest
	}
	if strings.Contains(p.host, "*") || p.host == "" {
		return originPattern{}, "a wildcard is valid only as the first label of the host"
	}
	return p, ""
}

// originPolicy holds the origins of one group.
type originPolicy struct {
	plain       []originPattern
	credentials []originPattern
}

// with returns a copy of the policy with the origins of one more option, so
// the routes before the option keep their policy.
func (p *originPolicy) with(o originsOption) *originPolicy {
	next := &originPolicy{}
	if p != nil {
		next.plain = append(next.plain, p.plain...)
		next.credentials = append(next.credentials, p.credentials...)
	}
	if o.credentials {
		next.credentials = append(next.credentials, o.patterns...)
	} else {
		next.plain = append(next.plain, o.patterns...)
	}
	return next
}

// match reports whether the policy lists the origin, and whether the origin
// keeps the cookies of the user.
func (p *originPolicy) match(origin string) (ok, credentials bool) {
	origin = strings.ToLower(origin)
	for _, c := range p.credentials {
		if c.match(origin) {
			return true, true
		}
	}
	for _, c := range p.plain {
		if c.match(origin) {
			return true, false
		}
	}
	return false, false
}

// crossOriginRequest reports whether a browser sent the request from a page
// of a different origin. A browser cannot set Sec-Fetch-Site from a script;
// a request with no Fetch Metadata is compared by its host.
func crossOriginRequest(r *http.Request, origin string) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return false
	case "":
		_, host, _ := strings.Cut(origin, "://")
		return !strings.EqualFold(host, r.Host)
	}
	return true
}

// serveListedOrigin answers a cross-origin request to a route of a group
// with AllowOrigins (REQ-ISL-22). It returns false for each other request,
// which then takes the cross-origin protection of SI-03.
func (a *App) serveListedOrigin(w http.ResponseWriter, r *http.Request) bool {
	if len(a.origins) == 0 {
		return false
	}
	_, pattern := a.mux.Handler(r)
	policy := a.origins[pattern]
	if policy == nil {
		return false
	}
	// The answer of the route depends on the origin of the request.
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin == "" || !crossOriginRequest(r, origin) {
		return false
	}
	ok, credentials := policy.match(origin)
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return true
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	if credentials {
		h.Set("Access-Control-Allow-Credentials", "true")
	} else {
		// The request of this origin has no session: the user comes
		// from the token of the request (SI-14).
		r.Header.Del("Cookie")
	}
	if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
		h.Set("Access-Control-Allow-Methods", r.Header.Get("Access-Control-Request-Method"))
		if headers := r.Header.Get("Access-Control-Request-Headers"); headers != "" {
			h.Set("Access-Control-Allow-Headers", headers)
		}
		h.Set("Access-Control-Max-Age", "600")
		h.Add("Vary", "Access-Control-Request-Method, Access-Control-Request-Headers")
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	// The group lists the origin, so the request is not a forged one.
	a.serve(w, r.WithContext(context.WithValue(r.Context(), csrfCheckedKey{}, true)))
	return true
}

// allowOrigins records the policy of a mounted route and answers OPTIONS on
// its path, for the CORS preflight.
func (a *App) allowOrigins(pattern string, policy *originPolicy) {
	if a.origins == nil {
		a.origins = map[string]*originPolicy{}
	}
	a.origins[pattern] = policy
	method, path, ok := strings.Cut(pattern, " ")
	if !ok || method == http.MethodOptions {
		return
	}
	options := http.MethodOptions + " " + path
	if a.patterns[options] {
		// A second method on the path, or an OPTIONS route of the app.
		// The policy of the last group that mounts the path holds.
		a.origins[options] = policy
		return
	}
	a.patterns[options] = true
	a.origins[options] = policy
	a.mux.HandleFunc(options, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}
