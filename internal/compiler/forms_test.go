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

// formNestedRoutesGo is the nested and repeated field module of the M5 test.
const formNestedRoutesGo = `package signup

import "github.com/alternayte/gx"

// Address is a nested form struct (REQ-FRM-08).
type Address struct {
	Street string   ` + "`form:\"street\"`" + `
	City   string   ` + "`form:\"city\"`" + `
	Lines  []string ` + "`form:\"lines\"`" + `
}

type Nested struct {
	gx.Route ` + "`POST /signup`" + `
	Name      string
	Address   Address
	Addresses []Address
	Tags      []string
}

func (in *Nested) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Name, gx.Required),
		gx.Field(&in.Address.Street, gx.Required),
	}
}

var nested = gx.Form(func(c *gx.Ctx, in *Nested) error {
	return c.Redirect(gx.URL("/done"))
}, NestedView)

type NestedPage struct {
	gx.Route ` + "`GET /nested`" + `
}

var NestedPageValue = gx.Page(func(c *gx.Ctx, in NestedPage) (NestedViewProps, error) {
	return nested.Props(&Nested{}), nil
}, NestedView)

var NestedRoutes = gx.Collect(NestedPageValue, nested)
`

const formNestedViewGx = `package signup

props {
  F NestedForm
}

<form {...p.F.Attrs()}>
  <input {...p.F.Name.Attrs()} />
  <input {...p.F.Address.Street.Attrs()} />
  {p.F.Addresses.Each(rowNode)}
  {p.F.Tags.Each(tagNode)}
</form>
`

const formNestedHelpersGo = `package signup

import "github.com/alternayte/gx"

// rowNode renders one repeated address row (REQ-FRM-08).
func rowNode(i int, a AddressForm) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "class", Value: "row"}},
		gx.Text(a.Street.Name),
		gx.Text(a.City.Name),
	)
}

// tagNode renders one repeated tag field.
func tagNode(i int, t gx.FormField[string]) gx.Node {
	return gx.El("input", t.Attrs())
}
`

const formNestedTestGo = `package signup

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestREQ_FRM_08_NestedBinding(t *testing.T) {
	var in Nested
	req := httptest.NewRequest("POST", "/signup", strings.NewReader(
		"name=N&address.street=S&address.city=C&addresses[0].street=A0&addresses[1].street=A1&addresses[0].lines[0]=l0&tags[0]=t0&tags[1]=t1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	errs, err := in.GxBindForm(req)
	if err != nil || len(errs) != 0 {
		t.Fatalf("GxBindForm: %v %v", err, errs)
	}
	if in.Name != "N" || in.Address.Street != "S" || in.Address.City != "C" {
		t.Fatalf("nested struct = %+v", in.Address)
	}
	if len(in.Addresses) != 2 || in.Addresses[0].Street != "A0" || in.Addresses[1].Street != "A1" {
		t.Fatalf("addresses = %+v", in.Addresses)
	}
	if len(in.Addresses[0].Lines) != 1 || in.Addresses[0].Lines[0] != "l0" {
		t.Fatalf("lines = %+v", in.Addresses[0].Lines)
	}
	if len(in.Tags) != 2 || in.Tags[0] != "t0" || in.Tags[1] != "t1" {
		t.Fatalf("tags = %+v", in.Tags)
	}

	f := in.GxFormValue(nil).(NestedForm)
	if f.Address.Street.Name != "address.street" || f.Address.Street.ID != "nested-address-street" {
		t.Fatalf("nested field = %+v", f.Address.Street)
	}
	rows := f.Addresses.Rows()
	if len(rows) != 2 || rows[0].Street.Name != "addresses[0].street" || rows[0].Street.ID != "nested-addresses-0-street" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[1].City.Name != "addresses[1].city" {
		t.Fatalf("second row = %+v", rows[1])
	}
	if tags := f.Tags.Rows(); len(tags) != 2 || tags[1].Name != "tags[1]" {
		t.Fatalf("tags = %+v", tags)
	}
}

func TestREQ_FRM_08_ConversionInsideRow(t *testing.T) {
	var in Nested
	req := httptest.NewRequest("POST", "/signup", strings.NewReader("name=N&address.street=S"))
	errs, err := in.GxBindForm(req)
	if err != nil || len(errs) != 0 {
		t.Fatalf("valid row: %v %v", err, errs)
	}
}
`

// TestREQ_FRM_08_NestedBinding covers nested structs, repeated fields and
// their generated form values (REQ-FRM-08).
func TestREQ_FRM_08_NestedBinding(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":                moduleWithGx(t),
		"signup/routes.go":      formNestedRoutesGo,
		"signup/NestedView.gx":  formNestedViewGx,
		"signup/helpers.go":     formNestedHelpersGo,
		"signup/nested_test.go": formNestedTestGo,
	})
	files := generateFiles(t, dir)
	routes := string(files[filepath.Join(dir, "signup/routes_gx.go")])
	for _, want := range []string{
		"type AddressForm struct {",
		"type NestedAddressesField struct {",
		"type NestedTagsField struct {",
		"func (f NestedAddressesField) Each(fn func(int, AddressForm) gx.Node) gx.Node {",
		`gx.FormIndexes(r, "addresses")`,
		`gx.FormText(r, ("address" + ".street"))`,
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("routes_gx.go lacks %q:\n%s", want, routes)
		}
	}
	writeGenerated(t, dir)
	cmd := exec.Command("go", "test", "-run", "TestREQ_FRM_08", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("nested binding test: %v\n%s", err, out)
	}
}

// formFilesRoutesGo is the upload module of the M5 test (REQ-FRM-09).
const formFilesRoutesGo = `package upload

import "github.com/alternayte/gx"

type Upload struct {
	gx.Route ` + "`POST /files`" + `
	Avatar gx.File
	Docs   []gx.File
}

func (in *Upload) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Avatar, gx.MaxSize(8), gx.Accept("image/*")),
		gx.Field(&in.Docs, gx.MaxSize(8)),
	}
}

var files = gx.Form(func(c *gx.Ctx, in *Upload) error {
	return c.Redirect(gx.URL("/done"))
}, FilesView)

type FilesPage struct {
	gx.Route ` + "`GET /files`" + `
}

var FilesPageValue = gx.Page(func(c *gx.Ctx, in FilesPage) (FilesViewProps, error) {
	return files.Props(&Upload{}), nil
}, FilesView)

var FilesRoutes = gx.Collect(FilesPageValue, files)
`

const formFilesViewGx = `package upload

props {
  F UploadForm
}

<form {...p.F.Attrs()}>
  <input {...p.F.Avatar.Attrs()} type="file" />
  <input {...p.F.Docs.Attrs()} type="file" />
</form>
`

const formFilesTestGo = `package upload

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
)

func upload(t *testing.T, fields map[string]struct{ name, mime, body string }) (*Upload, map[string]string, error) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for field, f := range fields {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf("form-data; name=%q; filename=%q", field, f.name))
		h.Set("Content-Type", f.mime)
		fw, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/files", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatal(err)
	}
	var in Upload
	errs, err := in.GxBindForm(req)
	return &in, errs, err
}

func TestREQ_FRM_09_Upload(t *testing.T) {
	in, errs, err := upload(t, map[string]struct{ name, mime, body string }{
		"avatar": {"a.png", "image/png", "png"},
		"docs":   {"d.txt", "text/plain", "doc"},
	})
	if err != nil || len(errs) != 0 {
		t.Fatalf("upload: %v %v", err, errs)
	}
	if in.Avatar.Name != "a.png" || in.Avatar.Size != 3 || in.Avatar.Type != "image/png" {
		t.Fatalf("avatar = %+v", in.Avatar)
	}
	if in.Avatar.Temp == "" {
		t.Fatalf("avatar has no temp file")
	}
	if _, err := os.Stat(in.Avatar.Temp); err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if data, err := os.ReadFile(in.Avatar.Temp); err != nil || string(data) != "png" {
		t.Fatalf("temp data = %q %v", data, err)
	}
	if len(in.Docs) != 1 || in.Docs[0].Name != "d.txt" {
		t.Fatalf("docs = %+v", in.Docs)
	}
}

func TestREQ_FRM_09_Oversize(t *testing.T) {
	_, errs, err := upload(t, map[string]struct{ name, mime, body string }{
		"avatar": {"a.png", "image/png", "0123456789"},
	})
	if err != nil {
		t.Fatalf("oversize bind error: %v", err)
	}
	if errs["avatar"] != "maxsize" {
		t.Fatalf("errs = %v, want avatar=maxsize", errs)
	}
}

func TestREQ_FRM_09_Accept(t *testing.T) {
	_, errs, err := upload(t, map[string]struct{ name, mime, body string }{
		"avatar": {"a.txt", "text/plain", "png"},
	})
	if err != nil {
		t.Fatalf("accept bind error: %v", err)
	}
	if errs["avatar"] != "accept" {
		t.Fatalf("errs = %v, want avatar=accept", errs)
	}
}
`

// TestREQ_FRM_09_FileBinding covers gx.File binding, limits and temp storage
// (REQ-FRM-09).
func TestREQ_FRM_09_FileBinding(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"upload/routes.go":     formFilesRoutesGo,
		"upload/FilesView.gx":  formFilesViewGx,
		"upload/files_test.go": formFilesTestGo,
	})
	files := generateFiles(t, dir)
	routes := string(files[filepath.Join(dir, "upload/routes_gx.go")])
	for _, want := range []string{
		"gx.ReadUploads(r,",
		"Enctype: \"multipart/form-data\"",
		"func (in *Upload) GxMaxUpload() int64 {",
		`gx.Attr{Key: "accept", Value: "image/*", Kind: gx.AttrText}`,
		"gx.File",
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("routes_gx.go lacks %q:\n%s", want, routes)
		}
	}
	writeGenerated(t, dir)
	cmd := exec.Command("go", "test", "-run", "TestREQ_FRM_09", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("file binding test: %v\n%s", err, out)
	}
}
