package gxcli_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

// TestREQ_TLS_03_LintAcceptsRouteTags covers gx lint on an app with routes:
// go vet reads the tag of a gx.Route field as a bad struct tag, and gx lint
// leaves that one finding out. Every other vet finding stays, also a bad
// tag on another field (REQ-TLS-03).
func TestREQ_TLS_03_LintAcceptsRouteTags(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"shop/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n\ntype Show struct {\n\tgx.Route `GET /products/{id}`\n\tID       int64\n\tTab      string `query:\"tab\"`\n}\n",
	})
	stderr := captureStderr(t, func() {
		if code := gxcli.Main([]string{"lint", dir}); code != 0 {
			t.Errorf("gx lint on route types exit = %d", code)
		}
	})
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("gx lint on a clean module prints:\n%s", stderr)
	}

	bad := scratchModule(t, map[string]string{
		"shop/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n",
		"shop/model.go":       "package shop\n\nimport \"fmt\"\n\ntype Item struct {\n\tName string `json:name`\n}\n\nfunc Print(i Item) { fmt.Printf(\"%d\\n\", i.Name) }\n",
	})
	stderr = captureStderr(t, func() {
		if code := gxcli.Main([]string{"lint", bad}); code == 0 {
			t.Error("gx lint passed a module with vet findings")
		}
	})
	for _, want := range []string{"model.go:6", "struct field tag `json:name`", "model.go:9", "Printf"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("gx lint lost the vet finding %q:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "route.go") {
		t.Fatalf("gx lint reports the route tag:\n%s", stderr)
	}

	// A module that does not compile fails with the compiler message.
	broken := scratchModule(t, map[string]string{
		"shop/model.go": "package shop\n\nfunc Broken() int { return \"no\" }\n",
	})
	stderr = captureStderr(t, func() {
		if code := gxcli.Main([]string{"lint", broken}); code == 0 {
			t.Error("gx lint passed a module that does not compile")
		}
	})
	if !strings.Contains(stderr, "model.go:3") {
		t.Fatalf("gx lint hides the compile error:\n%s", stderr)
	}
}
