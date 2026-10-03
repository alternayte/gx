package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestSI_04_SecretInSignal checks GX7002 for a secret signal.
func TestSI_04_SecretInSignal(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"cart/Cart.gx": "package cart\n\nimport \"github.com/alternayte/gx\"\n\nsignals {\n  Token gx.Secret = \"x\"\n}\n\n<p>x</p>\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeSecret)
	if !strings.Contains(d.Msg, "Token") {
		t.Fatalf("GX7002 message = %q", d.Msg)
	}
}

// TestSI_04_SecretInClientExpr checks GX7002 for a secret read.
func TestSI_04_SecretInClientExpr(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/types.go": "package cart\n\nimport \"github.com/alternayte/gx\"\n\ntype Model struct {\n\tToken gx.Secret\n}\n",
		"cart/Cart.gx":  "package cart\n\nimport \"github.com/alternayte/gx\"\n\nprops {\n  M Model\n}\n\ntoken := p.M.Token\n<p show={token == \"x\"}>x</p>\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeSecret)
	if !strings.Contains(d.Msg, "client expression") {
		t.Fatalf("GX7002 message = %q", d.Msg)
	}
}
