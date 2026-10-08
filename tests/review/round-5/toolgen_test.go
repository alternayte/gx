package round5_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// generateInto runs the generator and writes its files into the module.
func generateInto(t *testing.T, dir string) {
	t.Helper()
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("Generate diagnostics: %v", diags)
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const (
	notesRoute = "package route\n\nimport \"github.com/alternayte/gx\"\n\n" +
		"type Remove struct {\n\tgx.Route `DELETE /notes/{id}`\n\tID       int64\n\tReason   string\n}\n\n" +
		"func (in *Remove) Rules() gx.Rules {\n\treturn gx.Rules{gx.Field(&in.Reason, gx.Required)}\n}\n"
	notesGo = "package notes\n\nimport (\n\t\"app/notes/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
		"// Seen holds the inputs that the handler got.\nvar Seen []route.Remove\n\n" +
		"// Remove removes a note.\nvar Remove = gx.Action(func(c *gx.Ctx, in route.Remove) error {\n\tSeen = append(Seen, in)\n\treturn nil\n}).Tool()\n\n" +
		"var Routes = gx.Collect(Remove)\n"
	notesMain = "package main\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t\"net/http\"\n\t\"net/http/httptest\"\n\n\t\"app/notes\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
		"func main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", notes.Routes)\n" +
		"\t// The request of a user: the field is in the query of a DELETE.\n" +
		"\treq := httptest.NewRequest(\"DELETE\", \"https://app.example/notes/5?reason=old\", nil)\n\treq.Header.Set(\"Sec-Fetch-Site\", \"same-origin\")\n" +
		"\trec := httptest.NewRecorder()\n\tapp.ServeHTTP(rec, req)\n\tfmt.Printf(\"user: status %d, seen %+v\\n\", rec.Code, notes.Seen)\n" +
		"\tnotes.Seen = nil\n" +
		"\tanswer := app.CallTool(context.Background(), http.Header{}, \"notes_remove\", []byte(`{\"id\":5,\"reason\":\"old\"}`))\n" +
		"\tfmt.Printf(\"tool: error %v, text %q, seen %+v\\n\", answer.IsError, answer.Text, notes.Seen)\n}\n"
)

// TestREQ_AI_07_DeleteToolGetsItsArguments checks a tool whose action has
// the method DELETE and a field that is not in the path (REQ-AI-07: the
// endpoint "serves the same tools"; REQ-AI-06: "input schema from the input
// struct and its rules"; D-288: "A tool call is the request of its
// action"). The schema of the tool names the field, and a request of a user
// fills it from the query. The request of a tool call puts it into a form
// body, which net/http does not read for DELETE, so the action never gets
// the argument.
func TestREQ_AI_07_DeleteToolGetsItsArguments(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"notes/route/route.go": notesRoute,
		"notes/notes.go":       notesGo,
		"cmd/app/main.go":      notesMain,
	})
	generateInto(t, dir)
	cmd := exec.Command("go", "run", "./cmd/app")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "user: status 204, seen [{Route:{} ID:5 Reason:old}]") {
		t.Fatalf("the fixture is wrong: the request of a user did not run the action with its field:\n%s", text)
	}
	if !strings.Contains(text, "tool: error false") || !strings.Contains(text, "seen [{Route:{} ID:5 Reason:old}]\n") || strings.Count(text, "Reason:old") != 2 {
		t.Errorf("the tool call with {\"id\":5,\"reason\":\"old\"} did not run the DELETE action with its arguments:\n%s", text)
	}
}
