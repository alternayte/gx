package gx_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// signupFormStub is a hand-written stand-in for the generated form type.
type signupFormStub struct {
	gx.FormMeta
	Email gx.FormField[string]
	Age   gx.FormField[int]
}

type signupPropsStub struct{ F signupFormStub }

func (p *signupPropsStub) GxSetForm(v gx.FormValue) { p.F = v.(signupFormStub) }

type signupInStub struct {
	Email string
	Age   int
}

func (in *signupInStub) Pattern() string { return "POST /signup" }

func (in *signupInStub) GxBindForm(r *http.Request) (map[string]string, error) {
	errs := map[string]string{}
	if v := r.FormValue("email"); v != "" {
		in.Email = v
	}
	if v := r.FormValue("age"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			errs["age"] = "invalid"
		} else {
			in.Age = n
		}
	}
	return errs, nil
}

func (in *signupInStub) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Email, gx.Required, gx.Email),
		gx.Field(&in.Age, gx.Min(18)),
	}
}

func (in *signupInStub) GxNewForm() gx.FormInput { return &signupInStub{} }

func (in *signupInStub) GxRunForm(ctx *gx.Ctx, fn any) error {
	return fn.(func(*gx.Ctx, *signupInStub) error)(ctx, in)
}

func (in *signupInStub) GxFieldName(ptr any) string {
	switch ptr {
	case any(&in.Email):
		return "email"
	case any(&in.Age):
		return "age"
	}
	return ""
}

func (in *signupInStub) GxFormValue(errs map[string]string) gx.FormValue {
	f := signupFormStub{FormMeta: gx.FormMeta{Name: "signup", ID: "signup-form", Action: "/signup", Method: "POST"}}
	emailKey := errs["email"]
	f.Email = gx.FormField[string]{
		Name: "email", ID: "signup-email", Value: in.Email,
		ErrorKey: emailKey, Error: gx.Translate(emailKey, gx.DefaultMessage(emailKey)),
		Constraints: gx.Attrs{gx.Bool("required", true)},
		ValidateURL: "/signup?gx-validate=email",
	}
	ageKey := errs["age"]
	f.Age = gx.FormField[int]{
		Name: "age", ID: "signup-age", Value: in.Age,
		ErrorKey: ageKey, Error: gx.Translate(ageKey, gx.DefaultMessage(ageKey)),
	}
	return f
}

// signupViewStub renders the page that holds the form (REQ-FRM-02).
func signupViewStub(p signupPropsStub) gx.Node {
	return gx.El("main", nil,
		gx.El("form", p.F.Attrs(),
			gx.El("input", p.F.Email.Attrs()),
			gx.El("p", gx.Attrs{{Key: "id", Value: "signup-email-error"}, {Key: "role", Value: "alert"}}, gx.Text(p.F.Email.Error)),
			gx.El("p", gx.Attrs{{Key: "id", Value: "signup-age-error"}, {Key: "role", Value: "alert"}}, gx.Text(p.F.Age.Error)),
		),
	)
}

// TestREQ_FRM_02_FormHandler covers the form action: rules gate the handler,
// a rule failure and a FieldError re-render with the error, and the success
// path redirects.
func TestREQ_FRM_02_FormHandler(t *testing.T) {
	ran := 0
	form := gx.Form(func(c *gx.Ctx, in *signupInStub) error {
		ran++
		if in.Email == "taken@example.com" {
			return gx.FieldError(&in.Email, "email.taken")
		}
		return c.Redirect(gx.URL("/done"))
	}, signupViewStub)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/signup", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		form.ServeHTTP(rec, req)
		return rec
	}

	// An empty email fails the rule; the handler must not run.
	rec := post("email=&age=20")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid submit status = %d, want 422", rec.Code)
	}
	if ran != 0 {
		t.Fatalf("handler ran %d times before the rules passed", ran)
	}
	if !strings.Contains(rec.Body.String(), "This field is required.") {
		t.Fatalf("422 body lacks the rule error:\n%s", rec.Body.String())
	}

	// A value that fails a conversion is a field error, not a 400
	// (REQ-FRM-07).
	rec = post("email=a@b.co&age=abc")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Enter a valid value.") {
		t.Fatalf("conversion error: status %d body %q", rec.Code, rec.Body.String())
	}
	if ran != 0 {
		t.Fatalf("handler ran on a conversion error")
	}

	// The handler's field error re-renders the form.
	rec = post("email=taken@example.com&age=20")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("FieldError status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "email.taken") {
		t.Fatalf("FieldError body lacks the key:\n%s", rec.Body.String())
	}
	if ran != 1 {
		t.Fatalf("handler ran %d times, want 1", ran)
	}

	// A valid submit redirects with 303.
	rec = post("email=a@b.co&age=20")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/done" {
		t.Fatalf("valid submit = %d %q, want 303 /done", rec.Code, rec.Header().Get("Location"))
	}
	if ran != 2 {
		t.Fatalf("handler ran %d times, want 2", ran)
	}
}

// TestREQ_FRM_03_FieldAttrs covers the generated field attributes: name, id,
// value, constraints and the aria state (REQ-FRM-03, REQ-FRM-11).
func TestREQ_FRM_03_FieldAttrs(t *testing.T) {
	f := gx.FormField[string]{
		Name:        "email",
		ID:          "signup-email",
		Value:       "a@b.co",
		Error:       "This field is required.",
		ErrorKey:    "required",
		Constraints: gx.Attrs{gx.Bool("required", true), {Key: "type", Value: "email"}},
	}
	got := map[string]string{}
	for _, a := range f.Attrs() {
		if a.Kind == gx.AttrBool {
			got[a.Key] = "true"
			continue
		}
		got[a.Key] = a.Value
	}
	for key, want := range map[string]string{
		"name":             "email",
		"id":               "signup-email",
		"value":            "a@b.co",
		"required":         "true",
		"aria-invalid":     "true",
		"aria-describedby": "signup-email-error",
	} {
		if got[key] != want {
			t.Fatalf("attribute %s = %q, want %q", key, got[key], want)
		}
	}
	if inputType := f.FieldInputType("text"); inputType != "email" {
		t.Fatalf("FieldInputType = %q, want email", inputType)
	}
	if _, ok := got["type"]; ok {
		t.Fatalf("Attrs carries the rule type; the component sets it: %+v", got)
	}

	boolField := gx.FormField[bool]{Name: "terms", ID: "signup-terms", Value: true}
	attrs := boolField.Attrs()
	if attrValueOf(attrs, "checked") != "true" {
		t.Fatalf("bool field lacks checked: %+v", attrs)
	}
	if attrValueOf(attrs, "value") != "" {
		t.Fatalf("bool field has a value attribute: %+v", attrs)
	}
}

func attrValueOf(attrs gx.Attrs, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value
		}
	}
	return ""
}

// captureAdapter records the patches of a form or action answer.
type captureAdapter struct{ patches []gx.Patch }

func (a *captureAdapter) Name() string                         { return "capture" }
func (a *captureAdapter) Signals() bool                        { return true }
func (a *captureAdapter) Runtime() gx.Node                     { return nil }
func (a *captureAdapter) Assets() map[string][]byte            { return nil }
func (a *captureAdapter) ReadSignals(*http.Request, any) error { return nil }
func (a *captureAdapter) Respond(_ http.ResponseWriter, _ *http.Request, res *gx.Response) error {
	a.patches = append(a.patches, res.Patches...)
	return nil
}

// TestREQ_FRM_06_ValidateAction pins the live validation patch: the target
// is the field error element and a fix morphs it empty (REQ-FRM-06).
func TestREQ_FRM_06_ValidateAction(t *testing.T) {
	old := gx.AdapterOf(nil)
	adapter := &captureAdapter{}
	gx.SetAdapter(adapter)
	defer gx.SetAdapter(old)

	form := gx.Form(func(c *gx.Ctx, in *signupInStub) error { return nil }, signupViewStub)
	validate := func(value string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/signup?gx-validate=email", strings.NewReader("email="+value))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Datastar-Request", "true")
		rec := httptest.NewRecorder()
		form.ServeHTTP(rec, req)
		return rec
	}

	validate("nope")
	if len(adapter.patches) != 1 {
		t.Fatalf("patches = %d, want 1", len(adapter.patches))
	}
	patch, ok := adapter.patches[0].(gx.ElementPatch)
	if !ok {
		t.Fatalf("patch = %T, want gx.ElementPatch", adapter.patches[0])
	}
	if patch.Target != "#signup-email-error" {
		t.Fatalf("target = %q, want #signup-email-error", patch.Target)
	}
	if body := gx.String(patch.Node); !strings.Contains(body, "must be a valid email address") {
		t.Fatalf("patch body = %q", body)
	}

	adapter.patches = nil
	validate("a@b.co")
	patch, ok = adapter.patches[0].(gx.ElementPatch)
	if !ok || patch.Target != "#signup-email-error" {
		t.Fatalf("fix patch = %#v", adapter.patches)
	}
	if body := gx.String(patch.Node); strings.Contains(body, "must be") {
		t.Fatalf("fix patch still holds an error: %q", body)
	}
}

// TestREQ_FRM_10_Translator covers the message keys, the English defaults
// and a custom translator (REQ-FRM-10).
func TestREQ_FRM_10_Translator(t *testing.T) {
	if got := gx.Translate("required", gx.DefaultMessage("required")); got != "This field is required." {
		t.Fatalf("default message = %q", got)
	}
	if got := gx.DefaultMessage("email.taken"); got != "email.taken" {
		t.Fatalf("a key without a default = %q, want the key", got)
	}
	gx.SetTranslator(func(key, fallback string) string {
		if key == "email.taken" {
			return "That address is not free."
		}
		return fallback
	})
	defer gx.SetTranslator(nil)
	if got := gx.Translate("email.taken", gx.DefaultMessage("email.taken")); got != "That address is not free." {
		t.Fatalf("translated message = %q", got)
	}
	if got := gx.Translate("required", gx.DefaultMessage("required")); got != "This field is required." {
		t.Fatalf("fallback message = %q", got)
	}

	// The form value uses the translator for a rule key (REQ-FRM-10).
	in := &signupInStub{Email: "taken@example.com"}
	f := in.GxFormValue(map[string]string{"email": "email.taken"}).(signupFormStub)
	if f.Email.Error != "That address is not free." || f.Email.ErrorKey != "email.taken" {
		t.Fatalf("field error = %q key %q", f.Email.Error, f.Email.ErrorKey)
	}
}
