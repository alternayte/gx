package round1_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// The tests in this file generate code for a small app in a temp module and
// run the app's own tests against the generated code, as
// TestSI_06_MassAssignment and TestREQ_FRM_08_NestedBinding do.

const tick = "`"

// appRoutesGo holds a GET route with query defaults (REQ-RTE-03).
const appRoutesGo = `package route

import "github.com/alternayte/gx"

// List is a page of products.
type List struct {
	gx.Route ` + tick + `GET /products` + tick + `
	InStock  bool ` + tick + `query:"instock" default:"true"` + tick + `
	Offset   int  ` + tick + `query:"offset" default:"20"` + tick + `
}
`

const appRoutesTestGo = `package route

import (
	"net/http/httptest"
	"testing"
)

func TestInnerExplicitQueryValue(t *testing.T) {
	var def List
	if err := def.Bind(httptest.NewRequest("GET", "/products", nil)); err != nil {
		t.Fatal(err)
	}
	if !def.InStock || def.Offset != 20 {
		t.Fatalf("no query: got %+v, want the defaults InStock=true Offset=20", def)
	}

	var in List
	if err := in.Bind(httptest.NewRequest("GET", "/products?instock=false&offset=0", nil)); err != nil {
		t.Fatal(err)
	}
	if in.InStock {
		t.Errorf("?instock=false bound InStock=true: the default replaced the value in the request")
	}
	if in.Offset != 0 {
		t.Errorf("?offset=0 bound Offset=%d: the default replaced the value in the request", in.Offset)
	}
}
`

// appSignupGo holds a form with a repeated field (REQ-FRM-08).
const appSignupGo = `package signup

import "github.com/alternayte/gx"

// Address is one row of the form.
type Address struct {
	Street string
}

// Signup is the form input.
type Signup struct {
	gx.Route  ` + tick + `POST /signup` + tick + `
	Email     string
	Addresses []Address
}

// Rules checks the input.
func (in *Signup) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Email, gx.Required)}
}

var signup = gx.Form(func(c *gx.Ctx, in *Signup) error { return nil }, SignupView)
`

const appSignupViewGx = `package signup

props {
  F SignupForm
}

<form {...p.F.Attrs()}>
  <input {...p.F.Email.Attrs()} />
</form>
`

const appSignupTestGo = `package signup

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInnerRowIndex(t *testing.T) {
	// One small request. The row index is the largest int minus one.
	body := "email=a%40example.com&addresses[9223372036854775806].street=x"
	req := httptest.NewRequest("POST", "/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("the form handler panicked on a row index from the request: %v", r)
			}
		}()
		signup.ServeHTTP(rec, req)
	}()
	if rec.Code >= 500 {
		t.Fatalf("status = %d", rec.Code)
	}
}
`

// fmtBeforeGx is a component with inline markup after a block inside <pre>,
// where white space shows (REQ-AUT-17).
const fmtBeforeGx = `package fmtcase

props {
  Bold bool
}

<pre>
  if p.Bold {
    <b>b</b>
  }
<i>c</i>
</pre>
`

const fmtTestGo = `package fmtcase

import (
	"testing"

	"github.com/alternayte/gx"
)

func TestInnerFmtKeepsOutput(t *testing.T) {
	for _, bold := range []bool{false, true} {
		before := gx.String(Before(BeforeProps{Bold: bold}))
		after := gx.String(After(AfterProps{Bold: bold}))
		if before != after {
			t.Errorf("Bold=%v: gx fmt changed the rendered output\nbefore fmt: %q\nafter fmt:  %q", bold, before, after)
		}
	}
}
`

// generatedApp builds the shared app once and returns its test output.
func generatedApp(t *testing.T) string {
	t.Helper()
	appOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gx-review-app-")
		if err != nil {
			appErr = err.Error()
			return
		}
		defer os.RemoveAll(dir)
		formatted, diags := compiler.FormatSource("Before.gx", []byte(fmtBeforeGx))
		if len(diags) > 0 {
			appErr = "gx fmt diagnostics: " + diags[0].Msg
			return
		}
		writeModule(t, dir, map[string]string{
			"shop/route/routes.go":      appRoutesGo,
			"shop/route/routes_test.go": appRoutesTestGo,
			"signup/routes.go":          appSignupGo,
			"signup/SignupView.gx":      appSignupViewGx,
			"signup/signup_test.go":     appSignupTestGo,
			"fmtcase/Before.gx":         fmtBeforeGx,
			"fmtcase/After.gx":          string(formatted),
			"fmtcase/fmt_test.go":       fmtTestGo,
		})
		files, gdiags := compiler.Generate(dir)
		if len(gdiags) > 0 {
			appErr = "Generate diagnostics: " + gdiags[0].Msg
			return
		}
		for path, src := range files {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				appErr = err.Error()
				return
			}
			if err := os.WriteFile(path, src, 0o644); err != nil {
				appErr = err.Error()
				return
			}
		}
		appOut = goTestVerbose(dir)
	})
	if appErr != "" {
		t.Fatalf("the generated app did not build: %s", appErr)
	}
	return appOut
}

// REQ-RTE-03: "Query fields use query:\"name\" and an optional
// default:\"...\"."
//
// Defect: the generated Bind applies the default when the bound value is the
// zero value, not when the query has no value
// (internal/compiler/routes.go, emitted as `if in.InStock == false {
// in.InStock = true }`). A bool with default "true" can never be false, and
// ?offset=0 becomes 20. A filter link such as ?instock=false silently shows
// the default page. TestREQ_RTE_03_QueryDecode has no case where the request
// sends the zero value of a field that has another default.
func TestREQ_RTE_03_ExplicitQueryValueIsNotReplacedByDefault(t *testing.T) {
	requireInnerPass(t, generatedApp(t), "TestInnerExplicitQueryValue")
}

// REQ-FRM-08: "Nested structs and slices bind with indexed names
// (addresses[0].street)."
//
// Defect: the generated GxBindForm sizes the slice from the largest index in
// the request: `in.Addresses = make([]Address, gxIdx0[n-1]+1)`
// (internal/compiler/routes.go; gx.FormIndexes in form.go:603 accepts any
// non-negative int). The client picks the index, so one request with
// addresses[2000000000].street=x makes the server allocate gigabytes, and
// the largest index makes makeslice panic. The test uses the panic case so
// that it allocates nothing. TestREQ_FRM_08_NestedBinding binds indexes 0
// and 1 only.
func TestREQ_FRM_08_RowIndexFromRequestIsBounded(t *testing.T) {
	requireInnerPass(t, generatedApp(t), "TestInnerRowIndex")
}

// REQ-AUT-17: "gx fmt gives one canonical form and is idempotent. It never
// changes rendered output." Acceptance: "Render before and after fmt is
// equal."
//
// Defect: the formatter re-indents an element that follows a block
// (internal/compiler/format.go), also inside <pre>, where the indent is
// visible text. The rendered bytes change from "\n<i>" to "\n  <i>".
// TestREQ_AUT_17_FmtPreservesStructure compares parsed node dumps of two
// fixtures, not rendered output.
func TestREQ_AUT_17_FmtKeepsRenderedOutput(t *testing.T) {
	requireInnerPass(t, generatedApp(t), "TestInnerFmtKeepsOutput")
}

// SI-04: "Values of type gx.Secret cannot enter client expressions, signals,
// island props, fragments sent to other users or tool results. Compile error
// where visible, redaction and dev panic at runtime otherwise." secret.go
// says of the type: "It renders and marshals as [redacted]", and the docs
// say "When it reaches text or JSON by a different path, Gx writes
// [redacted]".
//
// Defect: codegen renders a value whose underlying type is string with a
// conversion, gx.Text(string(p.Token)) (internal/compiler/codegen.go,
// exprValue), which skips Secret.String. A secret in the text or in an
// attribute of a fragment goes to the browser in clear, in a page and in
// every c.Patch of that fragment. TestSI_04_SecretRedacted calls
// gx.TextValue and gx.JSON by hand and never renders a generated component.
func TestSI_04_SecretInFragmentIsNotRendered(t *testing.T) {
	const secret = "sk_live_REVIEW_ROUND_1"
	dir := scratchModule(t, map[string]string{
		"acct/Token.gx": "package acct\n\nprops {\n  Token gx.Secret\n}\n\n<span #tok title={p.Token}>{p.Token}</span>\n",
		"acct/token_test.go": `package acct

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

func TestInnerSecret(t *testing.T) {
	for name, node := range map[string]gx.Node{
		"component": Token(TokenProps{Token: "` + secret + `"}),
		"fragment":  TokenTok(TokenProps{Token: "` + secret + `"}),
	} {
		if out := gx.String(node); strings.Contains(out, "` + secret + `") {
			t.Errorf("%s: the gx.Secret value is in the HTML: %s", name, out)
		}
	}
}
`,
	})
	if diags := compiler.Check(dir); len(diagsFor(diags, "Token.gx")) > 0 {
		return // a compile error is a correct answer (GX7002)
	}
	generateInto(t, dir)
	out := goTestVerbose(dir)
	if strings.Contains(out, "undefined: TokenTok") {
		t.Fatalf("the test expects the fragment function TokenTok(TokenProps):\n%s", out)
	}
	requireInnerPass(t, out, "TestInnerSecret")
}
