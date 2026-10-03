package compiler_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// formRoutesGo is the form input of the form test module.
const formRoutesGo = `package signup

import "github.com/alternayte/gx"

type Plan string

const (
	Free Plan = "free"
	Pro  Plan = "pro"
)

type Signup struct {
	gx.Route ` + "`POST /signup`" + `
	Email string
	Age   int
	Plan  Plan
	Terms bool
}

func (in *Signup) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Email, gx.Required, gx.Email, gx.MaxLen(254)),
		gx.Field(&in.Age, gx.Min(18), gx.Max(120)),
		gx.Field(&in.Plan, gx.OneOf("free", "pro")),
		gx.Field(&in.Terms, gx.True("terms.required")),
	}
}

var signup = gx.Form(func(c *gx.Ctx, in *Signup) error {
	if in.Email == "taken@example.com" {
		return gx.FieldError(&in.Email, "email.taken")
	}
	return c.Redirect(gx.URL("/done"))
}, SignupView)

type SignupPage struct {
	gx.Route ` + "`GET /signup`" + `
}

var Page = gx.Page(func(c *gx.Ctx, in SignupPage) (SignupViewProps, error) {
	return signup.Props(&Signup{}), nil
}, SignupView)

var Routes = gx.Collect(Page, signup)
`

// formViewGx is the view that holds the generated form value.
const formViewGx = `package signup

props {
  F SignupForm
}

<main>
  <form {...p.F.Attrs()}>
    <label for={p.F.Email.ID}>Email</label>
    <input {...p.F.Email.Attrs()} />
    if p.F.Email.Error != "" {
      <p id="signup-email-error" role="alert">{p.F.Email.Error}</p>
    }
    <input {...p.F.Age.Attrs()} />
    if p.F.Age.Error != "" {
      <p id="signup-age-error" role="alert">{p.F.Age.Error}</p>
    }
    <input {...p.F.Terms.Attrs()} />
    <button type="submit">Join</button>
  </form>
</main>
`

const formPageTestGo = `package signup

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func postForm(target string, values url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", target, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	signup.ServeHTTP(rec, req)
	return rec
}

func TestREQ_FRM_07_Conversion(t *testing.T) {
	rec := postForm("/signup", url.Values{"email": {"a@b.co"}, "age": {"abc"}, "terms": {"on"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Enter a valid value.") {
		t.Fatalf("conversion = %d %q", rec.Code, rec.Body.String())
	}
}

func TestFormFlow(t *testing.T) {
	rec := postForm("/signup", url.Values{"email": {""}, "age": {"20"}, "plan": {"free"}, "terms": {"on"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "required") {
		t.Fatalf("invalid = %d %q", rec.Code, rec.Body.String())
	}
	rec = postForm("/signup", url.Values{"email": {"a@b.co"}, "age": {"7"}, "plan": {"free"}, "terms": {"on"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "too small") {
		t.Fatalf("min = %d %q", rec.Code, rec.Body.String())
	}
	rec = postForm("/signup", url.Values{"email": {"taken@example.com"}, "age": {"20"}, "plan": {"free"}, "terms": {"on"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "email.taken") {
		t.Fatalf("field error = %d %q", rec.Code, rec.Body.String())
	}
	rec = postForm("/signup", url.Values{"email": {"a@b.co"}, "age": {"20"}, "plan": {"free"}, "terms": {"on"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/done" {
		t.Fatalf("valid = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}
`

func formTree(t *testing.T, routes string) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"signup/routes.go":     routes,
		"signup/SignupView.gx": formViewGx,
		"signup/form_test.go":  formPageTestGo,
	})
}

// TestREQ_FRM_07_ConversionError covers the conversion error: the generated
// binder records a message key, and the form answers 422 with the field
// error, not 400 (REQ-FRM-07).
func TestREQ_FRM_07_ConversionError(t *testing.T) {
	dir := formTree(t, formRoutesGo)
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "signup/routes_gx.go")])
	if !strings.Contains(src, `errs["age"] = "invalid"`) {
		t.Fatalf("GxBindForm does not record a conversion error:\n%s", src)
	}
	writeGenerated(t, dir)
	cmd := exec.Command("go", "test", "-run", "TestREQ_FRM_07_Conversion", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("conversion error test: %v\n%s", err, out)
	}
}

// TestREQ_FRM_03_GeneratedForm covers the generated form type and its
// methods, and the generated property setter of the view (REQ-FRM-03).
func TestREQ_FRM_03_GeneratedForm(t *testing.T) {
	dir := formTree(t, formRoutesGo)
	files := generateFiles(t, dir)
	routes := string(files[filepath.Join(dir, "signup/routes_gx.go")])
	for _, want := range []string{
		"type SignupForm struct {",
		"gx.FormField[string]",
		"gx.FormField[int]",
		"gx.FormField[Plan]",
		"gx.FormField[bool]",
		"func (in *Signup) GxFormValue(errs map[string]string) gx.FormValue {",
		"func (in *Signup) GxBindForm(r *http.Request) (map[string]string, error) {",
		"func (in *Signup) GxFieldName(ptr any) string {",
		`case any(&in.Email):`,
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("routes_gx.go lacks %q:\n%s", want, routes)
		}
	}
	view := string(files[filepath.Join(dir, "signup/SignupView_gx.go")])
	want := "func (p *SignupViewProps) GxSetForm(v gx.FormValue) {"
	if !strings.Contains(view, want) {
		t.Fatalf("SignupView_gx.go lacks %q:\n%s", want, view)
	}
	writeGenerated(t, dir)
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
	_ = files
}

// TestREQ_FRM_04_Constraints covers the rule-to-constraint mapping
// (REQ-FRM-04).
func TestREQ_FRM_04_Constraints(t *testing.T) {
	dir := formTree(t, formRoutesGo)
	files := generateFiles(t, dir)
	routes := string(files[filepath.Join(dir, "signup/routes_gx.go")])
	for _, want := range []string{
		`Constraints: gx.Attrs{gx.Bool("required", true), gx.Attr{Key: "type", Value: "email", Kind: gx.AttrText}, gx.Attr{Key: "maxlength", Value: "254", Kind: gx.AttrText}}`,
		`Constraints: gx.Attrs{gx.Attr{Key: "min", Value: "18", Kind: gx.AttrText}, gx.Attr{Key: "max", Value: "120", Kind: gx.AttrText}}`,
		`Constraints: nil`,
		`Constraints: gx.Attrs{gx.Bool("required", true)}`,
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("routes_gx.go lacks constraint %q:\n%s", want, routes)
		}
	}
}

// TestREQ_FRM_03_RenameBreaksView pins that renaming an input field breaks
// the view at compile time (REQ-FRM-03).
func TestREQ_FRM_03_RenameBreaksView(t *testing.T) {
	bad := strings.Replace(formRoutesGo, "\tEmail string", "\tMail string", 1)
	bad = strings.Replace(bad, "&in.Email", "&in.Mail", 1)
	bad = strings.Replace(bad, "&in.Email", "&in.Mail", 1)
	bad = strings.Replace(bad, "in.Email ==", "in.Mail ==", 1)
	dir := formTree(t, bad)
	diags := compiler.Check(dir)
	found := false
	for _, d := range diags {
		if d.Code == compiler.CodeType && strings.Contains(d.Msg, "Email") && strings.HasSuffix(d.File, ".gx") {
			found = true
		}
	}
	if !found {
		t.Fatalf("rename was not a compile error in the view: %v", diags)
	}
}

// TestREQ_FRM_04_PatternConstraint covers a package-level regexp value
// (REQ-FRM-04).
func TestREQ_FRM_04_PatternConstraint(t *testing.T) {
	routes := strings.Replace(formRoutesGo,
		"func (in *Signup) Rules() gx.Rules {",
		"var skuRe = regexp.MustCompile(`^[A-Z]{3}$`)\n\nfunc (in *Signup) Rules() gx.Rules {", 1)
	routes = strings.Replace(routes,
		"gx.Field(&in.Email, gx.Required, gx.Email, gx.MaxLen(254)),",
		"gx.Field(&in.Email, gx.Required, gx.Pattern(skuRe)),", 1)
	routes = strings.Replace(routes, `import "github.com/alternayte/gx"`,
		"import (\n\t\"regexp\"\n\n\t\"github.com/alternayte/gx\"\n)", 1)
	dir := formTree(t, routes)
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "signup/routes_gx.go")])
	if !strings.Contains(src, `gx.Attr{Key: "pattern", Value: "^[A-Z]{3}$", Kind: gx.AttrText}`) {
		t.Fatalf("pattern constraint missing:\n%s", src)
	}
	if diags := compiler.Check(dir); len(diags) > 0 {
		t.Fatalf("check reported diagnostics: %v", diags)
	}
}
