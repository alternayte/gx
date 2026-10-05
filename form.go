package gx

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

// FormMeta is the generated form element state of an input type
// (REQ-FRM-03). The compiler embeds it in every <Type>Form value.
type FormMeta struct {
	// Name is the input type in lower-first form, for example "signup".
	Name string
	// ID is the id of the form element, for example "signup-form".
	ID string
	// Action is the form action URL, from the route pattern.
	Action string
	// Method is the HTTP method, for example "POST".
	Method string
	// Enctype is the form encoding when the form holds files
	// (REQ-FRM-09).
	Enctype string
}

// GxFormName implements FormValue.
func (m FormMeta) GxFormName() string { return m.Name }

// GxFormID implements FormValue.
func (m FormMeta) GxFormID() string { return m.ID }

// GxFormAction implements FormValue.
func (m FormMeta) GxFormAction() string { return m.Action }

// GxFormMethod implements FormValue.
func (m FormMeta) GxFormMethod() string { return m.Method }

// Attrs returns the attributes of the form element. The client runtime
// submits every form marked with data-gx-form through the adapter
// (REQ-FRM-05).
func (m FormMeta) Attrs() Attrs {
	out := Attrs{
		{Key: "id", Value: m.ID, Kind: AttrText},
		{Key: "method", Value: strings.ToLower(m.Method), Kind: AttrText},
		{Key: "action", Value: m.Action, Kind: AttrURL},
		{Key: "data-gx-form", Value: m.Name, Kind: AttrText},
	}
	if m.Enctype != "" {
		out = append(out, Attr{Key: "enctype", Value: m.Enctype, Kind: AttrText})
	}
	return out
}

// FormValue is implemented by every generated <Type>Form (REQ-FRM-03).
type FormValue interface {
	GxFormName() string
	GxFormID() string
	GxFormAction() string
	GxFormMethod() string
}

// FormProps is implemented by a generated view props struct that holds a
// form value (REQ-FRM-03). The generated GxSetForm fills that field.
type FormProps interface {
	GxSetForm(FormValue)
}

// FormInput is the generated interface of a form input type (REQ-FRM-02).
type FormInput interface {
	Pattern() string
	// GxNewForm returns a fresh input value.
	GxNewForm() FormInput
	GxBindForm(*http.Request) (map[string]string, error)
	Rules() Rules
	GxFormValue(map[string]string) FormValue
	GxFieldName(any) string
	// GxRunForm calls the user handler with the concrete input type.
	GxRunForm(*Ctx, any) error
}

// Translator turns a message key into a message (REQ-FRM-10). fallback is
// the English message of the key.
type Translator func(key, fallback string) string

var messageTranslator Translator

// SetTranslator sets the message translator of the process (REQ-FRM-10).
func SetTranslator(t Translator) { messageTranslator = t }

// Translate returns the message of key. Without a translator it returns
// fallback, the English default (REQ-FRM-10).
func Translate(key, fallback string) string {
	if messageTranslator != nil {
		if msg := messageTranslator(key, fallback); msg != "" {
			return msg
		}
	}
	return fallback
}

// defaultMessages holds the English default of every built-in rule key
// (REQ-FRM-10).
var defaultMessages = map[string]string{
	"required": "This field is required.",
	"email":    "Enter a valid email address.",
	"url":      "Enter a valid URL.",
	"minlen":   "This value is too short.",
	"maxlen":   "This value is too long.",
	"min":      "This value is too small.",
	"max":      "This value is too large.",
	"pattern":  "This value has the wrong format.",
	"oneof":    "Choose one of the allowed values.",
	"invalid":  "Enter a valid value.",
	"maxsize":  "The file is too large.",
	"accept":   "The file type is not allowed.",
}

// DefaultMessage returns the English message of a built-in rule key, or the
// key itself when the key has no default (REQ-FRM-10).
func DefaultMessage(key string) string {
	if msg, ok := defaultMessages[key]; ok {
		return msg
	}
	return key
}

// ValidateURL returns the live validation URL of one field (REQ-FRM-06).
func ValidateURL(action, field string) string {
	sep := "?"
	if strings.Contains(action, "?") {
		sep = "&"
	}
	return action + sep + "gx-validate=" + url.QueryEscape(field)
}

// ParseBool parses an HTML form boolean. "on" is the value of a checked
// checkbox without a value attribute.
func ParseBool(s string) (bool, error) {
	if s == "on" {
		return true, nil
	}
	return strconv.ParseBool(s)
}

// FieldNames returns the sorted field names of a field error map.
func FieldNames(errs map[string]string) []string {
	names := make([]string, 0, len(errs))
	for name := range errs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Field is one generated form control (REQ-FRM-03). The compiler writes the
// static parts; the form handler writes Error and ErrorKey.
type FormField[T any] struct {
	// Name is the form field name, for example "email".
	Name string
	// ID is the control id, for example "signup-email".
	ID string
	// Value is the bound value.
	Value T
	// Error is the translated error message, or "".
	Error string
	// ErrorKey is the stable message key of Error, for example "required".
	ErrorKey string
	// Constraints holds the native constraints of the rules
	// (REQ-FRM-04).
	Constraints Attrs
	// ValidateURL is the URL of the live validation action (REQ-FRM-06).
	ValidateURL string
	// Hint is the helper text of the control, or "".
	Hint string
}

// Attrs returns the control attributes: name, id, value, native constraints
// and the aria state (REQ-FRM-03, REQ-FRM-11).
func (f FormField[T]) Attrs() Attrs {
	out := make(Attrs, 0, len(f.Constraints)+6)
	out = append(out,
		Attr{Key: "name", Value: f.Name, Kind: AttrText},
		Attr{Key: "id", Value: f.ID, Kind: AttrText},
	)
	switch v := any(f.Value).(type) {
	case bool:
		out = append(out, Bool("checked", v))
	case File, []File:
		// A file input has no value attribute (REQ-FRM-09).
	default:
		out = append(out, Attr{Key: "value", Value: TextValue(f.Value), Kind: AttrText})
	}
	for _, c := range f.Constraints {
		// The input type comes from FieldInputType so a component can
		// pass its own type without a duplicate attribute.
		if c.Key == "type" {
			continue
		}
		out = append(out, c)
	}
	described := make([]string, 0, 2)
	if f.Hint != "" {
		described = append(described, f.ID+"-hint")
	}
	if f.Error != "" {
		described = append(described, f.ID+"-error")
		out = append(out, Attr{Key: "aria-invalid", Value: "true", Kind: AttrText})
	}
	if len(described) > 0 {
		out = append(out, Attr{Key: "aria-describedby", Value: strings.Join(described, " "), Kind: AttrText})
	}
	if f.ValidateURL != "" {
		out = append(out, Attr{Key: "data-gx-validate-url", Value: f.ValidateURL, Kind: AttrText})
	}
	return out
}

// FieldView is the non-generic view of a field, for field components that
// serve every field type (REQ-FRM-03).
type FieldView interface {
	Attrs() Attrs
	FieldName() string
	FieldID() string
	FieldValue() string
	FieldError() string
	FieldErrorKey() string
	FieldValidateURL() string
	// FieldInputType returns the rule-derived input type ("email", "url"),
	// or fallback (REQ-FRM-04).
	FieldInputType(fallback string) string
	// FieldHint returns the helper text, or "".
	FieldHint() string
}

// FieldName implements FieldView.
func (f FormField[T]) FieldName() string { return f.Name }

// FieldID implements FieldView.
func (f FormField[T]) FieldID() string { return f.ID }

// FieldValue implements FieldView.
func (f FormField[T]) FieldValue() string { return TextValue(f.Value) }

// FieldError implements FieldView.
func (f FormField[T]) FieldError() string { return f.Error }

// FieldErrorKey implements FieldView.
func (f FormField[T]) FieldErrorKey() string { return f.ErrorKey }

// FieldValidateURL implements FieldView.
func (f FormField[T]) FieldValidateURL() string { return f.ValidateURL }

// FieldHint implements FieldView.
func (f FormField[T]) FieldHint() string { return f.Hint }

// FieldInputType implements FieldView.
func (f FormField[T]) FieldInputType(fallback string) string {
	for _, c := range f.Constraints {
		if c.Key == "type" {
			return c.Value
		}
	}
	return fallback
}

// FieldControlAttrs returns the control attributes of one field with extra
// aria-describedby ids, for example a hint element (REQ-FRM-11).
func FieldControlAttrs(f FieldView, describedBy ...string) Attrs {
	attrs := f.Attrs()
	ids := make([]string, 0, len(describedBy)+2)
	existing := ""
	for _, a := range attrs {
		if a.Key == "aria-describedby" {
			existing = a.Value
			break
		}
	}
	if existing != "" {
		ids = append(ids, strings.Fields(existing)...)
	}
	for _, id := range describedBy {
		if id == "" {
			continue
		}
		found := false
		for _, have := range ids {
			if have == id {
				found = true
				break
			}
		}
		if !found {
			ids = append(ids, id)
		}
	}
	out := make(Attrs, 0, len(attrs)+1)
	for _, a := range attrs {
		if a.Key == "aria-describedby" {
			continue
		}
		out = append(out, a)
	}
	if len(ids) > 0 {
		out = append(out, Attr{Key: "aria-describedby", Value: strings.Join(ids, " "), Kind: AttrText})
	}
	return out
}

// fieldError is the error FieldError returns (REQ-FRM-02).
type fieldError struct {
	ptr any
	key string
}

func (e *fieldError) Error() string { return e.key }

// FieldError re-renders the form with key as the error of one input field
// (REQ-FRM-02). The field pointer names the field through the generated
// GxFieldName method.
func FieldError[T any](field *T, key string) error {
	return &fieldError{ptr: field, key: key}
}

// form is the typed form handler built by Form (REQ-FRM-02).
type form[In any, P any] struct {
	pattern string
	fn      func(*Ctx, In) error
	view    func(P) Node
	newIn   func() FormInput
}

// Form registers a form action for a route type (REQ-FRM-02). In is a
// pointer to the route input type:
//
//	gx.Form(func(c *gx.Ctx, in *Signup) error { ... }, SignupView)
//
// The view renders the page that holds the form. An invalid submit answers
// the full view with 422; with an adapter only the form element is patched
// (REQ-FRM-05).
func Form[In any, P any](fn func(*Ctx, In) error, view func(P) Node) *form[In, P] {
	var zero In
	in, ok := any(zero).(FormInput)
	if !ok {
		panic("gx: Form needs a pointer input with generated form methods; run gx generate")
	}
	var p P
	if _, ok := any(&p).(FormProps); !ok {
		panic("gx: form view props need a generated GxSetForm; the view props struct must hold the generated form type")
	}
	fresh := in.GxNewForm()
	return &form[In, P]{pattern: fresh.Pattern(), fn: fn, view: view, newIn: in.GxNewForm}
}

// Pattern implements Handler.
func (f *form[In, P]) Pattern() string { return f.pattern }

// exportFeature names the form for the export check (REQ-EXP-02).
func (f *form[In, P]) exportFeature() (kind string, serverOnly bool) { return "form", true }

// Props returns the view props of the form with the current values and no
// errors. A page that renders the form calls it (REQ-FRM-02). The page
// passes a pointer to a zero input:
//
//	signup.Props(&route.Signup{})
func (f *form[In, P]) Props(in In) P {
	return f.props(any(in).(FormInput), nil)
}

func (f *form[In, P]) props(in FormInput, errs map[string]string) P {
	fv := in.GxFormValue(errs)
	var p P
	any(&p).(FormProps).GxSetForm(fv)
	return p
}

// ServeHTTP binds the input, runs the rules, then the handler (REQ-FRM-02).
func (f *form[In, P]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	in := f.newIn()
	if mu, ok := in.(interface{ GxMaxUpload() int64 }); ok {
		if n := mu.GxMaxUpload(); n > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, n)
		}
	}
	r, scope := withUploadScope(r)
	defer scope.remove()
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, "gx: upload too large", http.StatusRequestEntityTooLarge)
				return
			}
			renderError(w, r, &BindError{Err: err})
			return
		}
	}
	errs, err := in.GxBindForm(r)
	if err != nil {
		renderError(w, r, &BindError{Err: err})
		return
	}
	if field := validateField(r); field != "" {
		f.validate(w, r, in, field)
		return
	}
	for _, v := range RunAllRulesContext(r.Context(), in) {
		// A conversion error keeps its key (REQ-FRM-07).
		if v.Field != "" && errs[v.Field] == "" {
			errs[v.Field] = v.Key
		}
	}
	ctx := &Ctx{W: w, R: r, res: &Response{}}
	if len(errs) == 0 {
		// The handler runs only when every rule passes (REQ-FRM-02).
		if err := in.GxRunForm(ctx, f.fn); err != nil {
			var fe *fieldError
			if errors.As(err, &fe) {
				if name := in.GxFieldName(fe.ptr); name != "" {
					errs[name] = fe.key
				} else {
					ctx.res.Err = err
					ctx.res.Patches = append(ctx.res.Patches, ToastPatch{Text: err.Error(), Kind: ToastError})
				}
			} else {
				var re *redirectError
				if errors.As(err, &re) {
					f.redirect(w, r, re.url)
					return
				}
				ctx.res.Err = err
				ctx.res.Patches = append(ctx.res.Patches, ToastPatch{Text: err.Error(), Kind: ToastError})
			}
		}
	}
	if len(errs) > 0 {
		f.invalid(w, r, in, errs)
		return
	}
	if len(ctx.res.Patches) > 0 {
		f.respond(w, r, ctx.res)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// respond sends the handler answer through the adapter, or as a plain
// redirect when the request is not an adapter request.
func (f *form[In, P]) respond(w http.ResponseWriter, r *http.Request, res *Response) {
	adapter := AdapterOf(r)
	if adapter == nil || !wantsEventStream(r) {
		for _, p := range res.Patches {
			if rp, ok := p.(RedirectPatch); ok {
				http.Redirect(w, r, rp.URL, http.StatusSeeOther)
				return
			}
		}
		http.Error(w, "gx: form answer needs an adapter", http.StatusInternalServerError)
		return
	}
	if err := adapter.Respond(w, r, res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// redirect answers a redirect error (REQ-FRM-05).
func (f *form[In, P]) redirect(w http.ResponseWriter, r *http.Request, url string) {
	adapter := AdapterOf(r)
	if adapter != nil && wantsEventStream(r) {
		if err := adapter.Respond(w, r, &Response{Patches: []Patch{RedirectPatch{URL: url}}}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// invalid re-renders the form (REQ-FRM-02). With an adapter it patches the
// form element; without one it answers 422 with the full view (REQ-FRM-05).
func (f *form[In, P]) invalid(w http.ResponseWriter, r *http.Request, in FormInput, errs map[string]string) {
	node := f.view(f.props(in, errs))
	id := in.GxFormValue(nil).GxFormID()
	adapter := AdapterOf(r)
	if adapter != nil && wantsEventStream(r) {
		el := findElementByID(node, id)
		if el == nil {
			http.Error(w, "gx: form element #"+id+" is missing from the form view", http.StatusInternalServerError)
			return
		}
		err := adapter.Respond(w, r, &Response{Patches: []Patch{ElementPatch{
			Mode:   ModeMorph,
			Target: "#" + id,
			Node:   el,
		}}})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = RenderRequest(w, r, node)
}

// validateField returns the field name of a live validation request
// (REQ-FRM-06), or "".
func validateField(r *http.Request) string {
	if field := r.URL.Query().Get("gx-validate"); field != "" {
		return field
	}
	return r.Header.Get("Gx-Validate")
}

// validate patches only the error element of one field (REQ-FRM-06).
func (f *form[In, P]) validate(w http.ResponseWriter, r *http.Request, in FormInput, field string) {
	adapter := AdapterOf(r)
	if adapter == nil || !wantsEventStream(r) {
		http.Error(w, "gx: live validation needs an adapter", http.StatusBadRequest)
		return
	}
	key, fallback := "", ""
	for _, v := range RunAllRulesContext(r.Context(), in) {
		if v.Field == field {
			key, fallback = v.Key, v.Message
			break
		}
	}
	fieldID := in.GxFormValue(nil).GxFormName() + "-" + field
	message := ""
	if key != "" {
		message = Translate(key, fallback)
	}
	// The error element always exists, so the patch morphs it (REQ-FRM-06).
	patch := ElementPatch{
		Mode:   ModeMorph,
		Target: "#" + fieldID + "-error",
		Node:   FieldErrorNode(fieldID, message),
	}
	if err := adapter.Respond(w, r, &Response{Patches: []Patch{patch}}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// FieldErrorNode renders the error element of one field (REQ-FRM-11). Field
// components render the same element so a live validation patch morphs it.
func FieldErrorNode(fieldID, message string) Node {
	return El("p", Attrs{
		{Key: "id", Value: fieldID + "-error", Kind: AttrText},
		{Key: "role", Value: "alert", Kind: AttrText},
	}, Text(message))
}

// findElementByID returns the first element with the given id.
func findElementByID(n Node, id string) *elNode {
	switch t := n.(type) {
	case *elNode:
		if attrValue(t, "id") == id {
			return t
		}
		for _, child := range t.children {
			if el := findElementByID(child, id); el != nil {
				return el
			}
		}
	case fragNode:
		for _, child := range t {
			if el := findElementByID(child, id); el != nil {
				return el
			}
		}
	}
	return nil
}

// FormText returns one request form value after parsing (REQ-FRM-08).
func FormText(r *http.Request, name string) string {
	parseRequestForm(r)
	return r.Form.Get(name)
}

// parseRequestForm parses a form body once, multipart included
// (REQ-FRM-09).
func parseRequestForm(r *http.Request) {
	if r.Form != nil {
		return
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		_ = r.ParseMultipartForm(32 << 20)
		return
	}
	_ = r.ParseForm()
}

// FormIndexes returns the sorted row indexes present for an indexed form
// name, for example 0 and 2 for addresses[0].street and addresses[2].city
// (REQ-FRM-08).
func FormIndexes(r *http.Request, prefix string) []int {
	parseRequestForm(r)
	seen := map[int]bool{}
	open := prefix + "["
	for name := range r.Form {
		if !strings.HasPrefix(name, open) {
			continue
		}
		rest := name[len(open):]
		end := strings.IndexByte(rest, ']')
		if end <= 0 {
			continue
		}
		n, err := strconv.Atoi(rest[:end])
		if err != nil || n < 0 {
			continue
		}
		seen[n] = true
	}
	out := make([]int, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// FieldID returns the element id of one form field, for example
// ("signup", "addresses[0].street") gives "signup-addresses-0-street"
// (REQ-FRM-03).
func FieldID(form, path string) string {
	var b strings.Builder
	b.WriteString(form)
	b.WriteByte('-')
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '.', '[':
			b.WriteByte('-')
		case ']':
			// Rows and nesting separate two id parts already.
		default:
			b.WriteByte(path[i])
		}
	}
	return b.String()
}

// EachRow renders one node per row of a slice field (REQ-FRM-08).
func EachRow[T any](rows []T, fn func(int, T) Node) Node {
	var b Builder
	for i, v := range rows {
		b.Add(fn(i, v))
	}
	return b.Node()
}

// File is one uploaded file (REQ-FRM-09). The upload streams to a temp file
// during binding; the framework removes it after the handler returns.
type File struct {
	// Name is the client file name.
	Name string
	// Size is the number of bytes stored.
	Size int64
	// Type is the content type, for example "image/png".
	Type string
	// Temp is the path of the temp file.
	Temp string
}

// Open opens the uploaded file for reading.
func (f File) Open() (io.ReadCloser, error) { return os.Open(f.Temp) }

// Remove deletes the temp file.
func (f File) Remove() error { return os.Remove(f.Temp) }

// MaxSize limits an uploaded file in bytes (REQ-FRM-09).
func MaxSize(n int64) Rule {
	return Rule{key: "maxsize", check: func(v any) error {
		for _, f := range filesOf(v) {
			if f.Temp == "" {
				continue // no upload
			}
			if f.Size > n {
				return violation("maxsize", "the file is too large")
			}
		}
		return nil
	}}
}

// Accept limits the content types of an uploaded file (REQ-FRM-09).
func Accept(types ...string) Rule {
	return Rule{key: "accept", check: func(v any) error {
		for _, f := range filesOf(v) {
			if f.Temp == "" {
				continue // no upload
			}
			if !accepts(types, f.Type) {
				return violation("accept", "the file type is not allowed")
			}
		}
		return nil
	}}
}

func filesOf(v any) []File {
	switch x := v.(type) {
	case File:
		return []File{x}
	case []File:
		return x
	}
	return nil
}

// accepts reports whether a content type matches one accept pattern.
func accepts(patterns []string, contentType string) bool {
	if contentType == "" {
		return len(patterns) == 0
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	for _, p := range patterns {
		if p == mediaType {
			return true
		}
		if prefix, ok := strings.CutSuffix(p, "/*"); ok && strings.HasPrefix(mediaType, prefix+"/") {
			return true
		}
	}
	return false
}

// uploadScope collects the temp files of one request (REQ-FRM-09).
type uploadScope struct{ paths []string }

type uploadScopeKey struct{}

// withUploadScope installs the temp-file collector of one request.
func withUploadScope(r *http.Request) (*http.Request, *uploadScope) {
	scope := &uploadScope{}
	return r.WithContext(context.WithValue(r.Context(), uploadScopeKey{}, scope)), scope
}

func (s *uploadScope) remove() {
	for _, path := range s.paths {
		_ = os.Remove(path)
	}
}

// ReadUploads streams every part of one form name to a temp file and
// enforces the size and type limits before the handler runs (REQ-FRM-09).
func ReadUploads(r *http.Request, name string, maxSize int64, patterns []string) ([]File, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	headers := r.MultipartForm.File[name]
	out := make([]File, 0, len(headers))
	for _, fh := range headers {
		file, err := storeUpload(r, fh, maxSize, patterns)
		if err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	return out, nil
}

// storeUpload streams one multipart part to a temp file.
func storeUpload(r *http.Request, fh *multipart.FileHeader, maxSize int64, patterns []string) (File, error) {
	if maxSize > 0 && fh.Size > maxSize {
		return File{}, violation("maxsize", "the file is too large")
	}
	mediaType, _, _ := mime.ParseMediaType(fh.Header.Get("Content-Type"))
	if len(patterns) > 0 && !accepts(patterns, mediaType) {
		return File{}, violation("accept", "the file type is not allowed")
	}
	src, err := fh.Open()
	if err != nil {
		return File{}, err
	}
	defer src.Close()
	tmp, err := os.CreateTemp("", "gx-upload-*")
	if err != nil {
		return File{}, err
	}
	n, err := io.Copy(tmp, src)
	closeErr := tmp.Close()
	if err != nil {
		_ = os.Remove(tmp.Name())
		return File{}, err
	}
	if closeErr != nil {
		_ = os.Remove(tmp.Name())
		return File{}, closeErr
	}
	if maxSize > 0 && n > maxSize {
		_ = os.Remove(tmp.Name())
		return File{}, violation("maxsize", "the file is too large")
	}
	if scope, ok := r.Context().Value(uploadScopeKey{}).(*uploadScope); ok {
		scope.paths = append(scope.paths, tmp.Name())
	}
	return File{Name: fh.Filename, Size: n, Type: mediaType, Temp: tmp.Name()}, nil
}

// ViolationKey returns the message key of a field violation, or "invalid"
// (REQ-FRM-09).
func ViolationKey(err error) string {
	var fv *FieldViolation
	if errors.As(err, &fv) {
		return fv.Key
	}
	return "invalid"
}
