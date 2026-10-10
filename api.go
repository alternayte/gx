package gx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// API marks the action for the JSON wire (REQ-ACT-19). A request to the URL
// of the action with "Accept: application/json" then gets JSON and no patch.
// The input comes from the JSON body, the path and the query, and passes the
// rules. The answer is the value of ToolResult, or status 204 with none.
// Only an action with API answers JSON; each other action answers 406.
func (a *action[In]) API() *action[In] {
	a.api = newTool[In](nil)
	return a
}

// wantsJSON reports whether a request asks for the JSON wire: its Accept
// header names application/json, and no type of a page or of an adapter. A
// widget and an adapter also name JSON; they have their own answers.
func wantsJSON(r *http.Request) bool {
	if isWidgetRequest(r) || wantsEventStream(r) {
		return false
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}

// apiError is the JSON of a failed call.
type apiError struct {
	Key     string `json:"key"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message,omitempty"`
}

func writeAPIErrors(w http.ResponseWriter, status int, errs ...apiError) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Errors []apiError `json:"errors"`
	}{errs})
}

// serveJSON answers a request of the JSON wire (REQ-ACT-19). It gives the
// arguments to the action as a tool call does: each one goes to the part of
// the request that the binder reads, and the action runs with its rules and
// writes no answer for a page.
func (a *action[In]) serveJSON(w http.ResponseWriter, r *http.Request) {
	if a.api == nil {
		writeAPIErrors(w, http.StatusNotAcceptable, apiError{Key: "gx.not_api", Message: "this action has no JSON answer"})
		return
	}
	args := map[string]any{}
	if r.Body != nil && r.Body != http.NoBody {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err == nil && len(strings.TrimSpace(string(body))) > 0 {
			// A form of a different site cannot send application/json
			// with no preflight (SI-03).
			if ct, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";"); !strings.EqualFold(strings.TrimSpace(ct), "application/json") {
				writeAPIErrors(w, http.StatusUnsupportedMediaType, apiError{Key: "gx.not_json", Message: "the body of a JSON call is application/json"})
				return
			}
			args, err = toolArgs(body)
		}
		if err != nil {
			writeAPIErrors(w, http.StatusBadRequest, apiError{Key: "gx.bad_input", Message: "the body is not a JSON object"})
			return
		}
	}
	tc := &toolCall{signals: map[string]any{}}
	form := url.Values{}
	for _, f := range a.api.info.Fields {
		v, ok := args[f.Name]
		if !ok && f.In == "signal" && r.URL.Query().Has(f.Name) {
			// A method with no body has each argument in the query,
			// as the client of gx api writes it: the text of the
			// value. The schema of the input gives the type. A string
			// stays the text: the term "123" is not the number 123.
			text := r.URL.Query().Get(f.Name)
			if schemaType(a.api.info.Schema, f.Name) == "string" {
				v = text
			} else if err := json.Unmarshal([]byte(text), &v); err != nil {
				v = text
			}
			ok = true
		}
		if !ok {
			continue
		}
		switch f.In {
		case "path", "query":
			// The URL of the request has these.
		case "signal":
			tc.signals[f.Name] = v
		default:
			flattenArg(form, f.Name, v)
		}
	}
	in := r.Clone(context.WithValue(r.Context(), toolCallKey{}, tc))
	in.Header.Del("Accept")
	in.Form, in.PostForm, in.MultipartForm = nil, nil, nil
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		encoded := form.Encode()
		in.Body = io.NopCloser(strings.NewReader(encoded))
		in.ContentLength = int64(len(encoded))
		in.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	default:
		// net/http reads a form body only for POST, PUT and PATCH. The
		// binder reads the form values of each other method from the
		// query.
		u := *r.URL
		query := u.Query()
		for name, values := range form {
			query[name] = values
		}
		u.RawQuery = query.Encode()
		in.URL = &u
		in.Body = http.NoBody
		in.Header.Del("Content-Type")
	}
	rec := &statusRecorder{header: http.Header{}}
	a.ServeHTTP(rec, in)

	switch {
	case !tc.done:
		// The cross-origin check answered in the place of the action.
		writeAPIErrors(w, rec.status(), apiError{Key: "gx.refused"})
	case tc.bad:
		writeAPIErrors(w, http.StatusBadRequest, apiError{Key: "gx.bad_input", Message: "the arguments do not have the form of the input"})
	case len(tc.errs) > 0:
		names := make([]string, 0, len(tc.errs))
		for name := range tc.errs {
			names = append(names, name)
		}
		sort.Strings(names)
		errs := make([]apiError, len(names))
		for i, name := range names {
			errs[i] = apiError{Key: tc.errs[name], Field: name, Message: Translate(tc.errs[name], DefaultMessage(tc.errs[name]))}
		}
		writeAPIErrors(w, http.StatusUnprocessableEntity, errs...)
	case tc.res == nil:
		writeAPIErrors(w, http.StatusInternalServerError, apiError{Key: "gx.error"})
	case tc.res.Err != nil:
		writeAPIErrors(w, statusFor(tc.res.Err), apiError{Key: "gx.error", Message: tc.res.Err.Error()})
	case tc.res.hasTool && hasSecretKey(tc.res.tool):
		// The JSON of a map has its keys as text, so this secret has no
		// redacted form (SI-04).
		writeAPIErrors(w, http.StatusInternalServerError, apiError{Key: "gx.error", Message: "the result has a gx.Secret as the key of a map"})
	case tc.res.hasTool:
		data, err := json.Marshal(tc.res.tool)
		if err != nil {
			writeAPIErrors(w, http.StatusInternalServerError, apiError{Key: "gx.error", Message: "the result is not JSON"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(append(data, '\n'))
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// schemaType returns the JSON type of one property of an input schema, or
// "" when the schema does not give one type.
func schemaType(schema, name string) string {
	var doc struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
	}
	if json.Unmarshal([]byte(schema), &doc) != nil {
		return ""
	}
	t, _ := doc.Properties[name].Type.(string)
	return t
}
