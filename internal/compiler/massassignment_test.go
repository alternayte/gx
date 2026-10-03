package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestSI_06_MassAssignment checks the binder fills declared fields only
// (SI-06).
func TestSI_06_MassAssignment(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		"cart/routes.go": "package cart\n\nimport \"github.com/alternayte/gx\"\n\n" +
			"type Update struct {\n" +
			"\tgx.Route `POST /cart/update`\n" +
			"\tNote  string `form:\"note\"`\n" +
			"\tAdmin bool   `form:\"admin\" bind:\"-\"`\n" +
			"\thidden string\n" +
			"}\n",
		"cart/action.go": "package cart\n\nimport \"github.com/alternayte/gx\"\n\nvar update = gx.Action(func(c *gx.Ctx, in Update) error { return nil })\n",
		"cart/bind_test.go": "package cart\n\nimport (\n\t\"net/http/httptest\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestBind(t *testing.T) {\n" +
			"\treq := httptest.NewRequest(\"POST\", \"/cart/update\", strings.NewReader(\"note=hi&admin=true&hidden=x\"))\n" +
			"\treq.Header.Set(\"Content-Type\", \"application/x-www-form-urlencoded\")\n" +
			"\tvar in Update\n" +
			"\tif err := in.Bind(req); err != nil { t.Fatal(err) }\n" +
			"\tif in.Note != \"hi\" { t.Fatalf(\"note = %q\", in.Note) }\n" +
			"\tif in.Admin { t.Fatal(\"bind:\\\"-\\\" field was bound\") }\n" +
			"\tif in.hidden != \"\" { t.Fatal(\"unexported field was bound\") }\n" +
			"\t}\n",
	})
	files := generateFiles(t, dir)
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
}
