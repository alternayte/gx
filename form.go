package gx

import (
	"errors"
	"net/http"
	"net/url"
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
	return Attrs{
		{Key: "id", Value: m.ID, Kind: AttrText},
		{Key: "method", Value: strings.ToLower(m.Method), Kind: AttrText},
		{Key: "action", Value: m.Action, Kind: AttrURL},
		{Key: "data-gx-form", Value: m.Name, Kind: AttrText},
	}
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
	if b, ok := any(f.Value).(bool); ok {
		out = append(out, Bool("checked", b))
	} else {
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

// FieldInputType implements FieldView.
func (f FormField[T]) FieldInputType(fallback string) string {
	for _, c := range f.Constraints {
		if c.Key == "type" {
			return c.Value
		}
	}
	return fallback
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
					ctx.res.Patches = append(ctx.res.Patches, ToastPatch{Text: err.Error()})
				}
			} else {
				var re *redirectError
				if errors.As(err, &re) {
					f.redirect(w, r, re.url)
					return
				}
				ctx.res.Err = err
				ctx.res.Patches = append(ctx.res.Patches, ToastPatch{Text: err.Error()})
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
