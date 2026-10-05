package round1_test

import (
	"path/filepath"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// SI-02: "href, src, action and formaction never take a dynamic string."
// Never-list (§2.2): "Accept a dynamic string in href, src, action or
// formaction." REQ-RTE-05: "GX2011 for a dynamic string."
//
// Defect: isURLAttr (internal/compiler/codegen.go:1349) compares the
// attribute name as written. HTML attribute names are case-insensitive, so
// <a HREF={p.Link}> is the href attribute for the browser, but the checker
// sees a plain attribute: no GX2011, and the generated code writes the
// string with Kind gx.AttrText. TestSI_02_DynamicURLRejected tries the
// lower-case name only.
func TestSI_02_DynamicURLRejectedInAnyLetterCase(t *testing.T) {
	byFile := checkEach(t, map[string]string{
		"Upper": "package card\n\nprops {\n  Link string\n}\n\n<a HREF={p.Link}>x</a>\n",
		"Title": "package card\n\nprops {\n  Link string\n}\n\n<a Href={p.Link}>x</a>\n",
		"Src":   "package card\n\nprops {\n  Link string\n}\n\n<iframe SRC={p.Link}></iframe>\n",
		"Form":  "package card\n\nprops {\n  Link string\n}\n\n<button FormAction={p.Link}>x</button>\n",
	})
	for name, diags := range byFile {
		if !hasCode(diags, "GX2011") {
			t.Errorf("%s.gx, a URL attribute in another letter case with a string prop: diagnostics = %v, want GX2011", name, diags)
		}
	}
}

// REQ-AUT-12: "Dynamic values in on* HTML event attributes are rejected. ...
// GX2007 for a dynamic onclick."
//
// Defect: the check is strings.HasPrefix(a.Name, "on")
// (internal/compiler/checktypes.go:549), which is case-sensitive.
// <button ONCLICK={p.Code}> compiles and the string becomes script.
// TestREQ_AUT_12_EventAttributeRejected tries the lower-case name only.
func TestREQ_AUT_12_EventAttributeRejectedInAnyLetterCase(t *testing.T) {
	byFile := checkEach(t, map[string]string{
		"Upper": "package card\n\nprops {\n  Code string\n}\n\n<button ONCLICK={p.Code}>x</button>\n",
		"Title": "package card\n\nprops {\n  Code string\n}\n\n<button OnMouseOver={p.Code}>x</button>\n",
	})
	for name, diags := range byFile {
		if !hasCode(diags, "GX2007") {
			t.Errorf("%s.gx, an event attribute in another letter case with a string prop: diagnostics = %v, want GX2007", name, diags)
		}
	}
}

// SI-02 and the never-list, as above. §6.1: "attr:<name> ... Set an
// attribute."
//
// Defect: the URL check skips every directive
// (internal/compiler/checktypes.go:546, isDirective is true for any name
// with a colon), and codegen tests isURLAttr on the full name "attr:href"
// (codegen.go:631). <a attr:href={p.Link}> therefore compiles to
// data-attr:href with the string inlined as JSON, and <a attr:href={$Link}>
// to the signal. Datastar sets href to that string in the browser, so a
// javascript: address from data runs on click. No test covers attr: with a
// URL attribute.
func TestSI_02_AttrDirectiveCannotSetURLAttributeFromString(t *testing.T) {
	files := map[string]string{
		"ui/card/Server.gx": "package card\n\nprops {\n  Link string\n}\n\n<a attr:href={p.Link}>x</a>\n",
		"ui/card/Signal.gx": "package card\n\nprops {\n  Link string\n}\nsignals { Target string = p.Link }\n\n<a attr:href={$Target}>x</a>\n",
		"ui/card/Form.gx":   "package card\n\nprops {\n  Link string\n}\n\n<form attr:action={p.Link}></form>\n",
	}
	dir := scratchModule(t, files)
	diags := compiler.Check(dir)
	generated, _ := compiler.Generate(dir)
	for _, name := range []string{"Server", "Signal", "Form"} {
		if len(diagsFor(diags, name+".gx")) > 0 {
			continue // rejected: correct
		}
		code := string(generated[filepath.Join(dir, "ui/card/"+name+"_gx.go")])
		t.Errorf("%s.gx: a dynamic string reaches a URL attribute through attr: with no diagnostic; generated:\n%s", name, lineWith(code, "data-attr:"))
	}
}

// REQ-AUT-12: "Dynamic values in on* HTML event attributes are rejected."
//
// Defect: the same directive skip lets attr:onclick through. The value of
// the signal, which starts as a server string and which the browser can
// change, becomes the onclick attribute: Datastar calls
// setAttribute("onclick", value).
func TestREQ_AUT_12_AttrDirectiveCannotSetEventAttribute(t *testing.T) {
	src := "package card\n\nprops {\n  Code string\n}\nsignals { Handler string = p.Code }\n\n<button attr:onclick={$Handler}>x</button>\n"
	dir := scratchModule(t, map[string]string{"ui/card/Card.gx": src})
	if diags := compiler.Check(dir); len(diags) == 0 {
		files, _ := compiler.Generate(dir)
		code := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
		t.Fatalf("attr:onclick={$Handler} compiles with no diagnostic; generated:\n%s", lineWith(code, "data-attr:onclick"))
	}
}

// REQ-ACT-13: "Client-expression types: bool, string, int, float64 and
// signal refs. ... Anything else is GX4007." DR-05: "Client expressions
// allow only types and operators where Go and JS give the same result."
//
// Defect: checkClientSite (internal/compiler/clienttranspile.go:275) checks
// only the operands of == and !=. A uint8 signal is accepted, and
// `$Left - 2 > 0` is true in Go for Left = 1 (255 > 0) and false in the
// browser (-1 > 0). TestREQ_ACT_13_BadCompare covers == on a struct only.
func TestREQ_ACT_13_UnsupportedSignalTypeIsGX4007(t *testing.T) {
	byFile := checkEach(t, map[string]string{
		"Uint8": "package card\n\nsignals { Left uint8 = 1 }\n\n<p show={$Left - 2 > 0}>x</p>\n",
		"Uint":  "package card\n\nsignals { Left uint = 1 }\n\n<p show={$Left - 2 > 0}>x</p>\n",
	})
	for name, diags := range byFile {
		if !hasCode(diags, "GX4007") {
			t.Errorf("%s.gx, an unsigned signal in a client expression: diagnostics = %v, want GX4007", name, diags)
		}
	}
}

// DR-05 and REQ-ACT-13, as above.
//
// Defect: jsOp (internal/compiler/clienttranspile.go:585) passes every
// operator it does not know to JavaScript unchanged. `$N << 31` is
// 2147483648 in Go and -2147483648 in JavaScript (32-bit shift); `&^` and
// the unary `^` are not JavaScript at all, so the attribute throws in the
// browser. TestREQ_ACT_13_Differential runs + - * / % and the comparisons
// only.
func TestREQ_ACT_13_OperatorWithDifferentJSResultIsRejected(t *testing.T) {
	exprs := map[string]string{"Shift": "$N << 31 > 0", "AndNot": "$N &^ 3 > 0", "Xor": "^$N > 0"}
	sources := map[string]string{}
	for name, expr := range exprs {
		sources[name] = "package card\n\nsignals { N int = 1 }\n\n<p show={" + expr + "}>x</p>\n"
	}
	for name, diags := range checkEach(t, sources) {
		if len(diags) == 0 {
			t.Errorf("show={%s} compiles with no diagnostic", exprs[name])
		}
	}
}
