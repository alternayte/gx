package compiler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_CNT_10_LinkCheck covers the content link check: a broken page
// link or heading anchor is GX8003, and external links are checked only on
// request (REQ-CNT-10).
func TestREQ_CNT_10_LinkCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broken" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dir := writeTree(t, map[string]string{
		"go.mod":                 moduleWithGx(t),
		"docs/content.go":        "package docs\n\nimport \"github.com/alternayte/gx\"\n\ntype DocMeta struct {\n\tTitle string `yaml:\"title\"`\n}\n\nvar Docs = gx.Collection[DocMeta](\"content/docs\").Components()\n",
		"content/docs/index.md":  "# Home\n\n[guides](/guides/)\n[bad](/missing/)\n[external ok](" + srv.URL + "/ok)\n[external bad](" + srv.URL + "/broken)\n",
		"content/docs/guides.md": "# Guides\n\n## Install\n\n[home](/)\n[anchor](#install)\n[bad anchor](#nope)\n[file](./guides.md#install)\n",
	})

	// Internal links only: no external request runs.
	diags := compiler.Check(dir)
	var msgs []string
	for _, d := range diags {
		if d.Code == compiler.CodeContentLink {
			msgs = append(msgs, d.Msg)
			if d.Line == 0 || !strings.HasSuffix(d.File, ".md") {
				t.Errorf("GX8003 position = %s:%d", d.File, d.Line)
			}
		}
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{"/missing/", "#nope"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no GX8003 for %q in:\n%s", want, joined)
		}
	}
	for _, bad := range []string{"/guides/", "#install", "./guides.md"} {
		if strings.Contains(joined, bad) {
			t.Errorf("GX8003 reported the good link %q in:\n%s", bad, joined)
		}
	}
	if strings.Contains(joined, srv.URL) {
		t.Errorf("external links were checked without the option:\n%s", joined)
	}

	// External links on request.
	diags = compiler.CheckWith(dir, compiler.CheckOptions{ExternalLinks: true, Client: srv.Client()})
	found := ""
	for _, d := range diags {
		if d.Code == compiler.CodeContentLink {
			found += d.Msg + "\n"
		}
	}
	if !strings.Contains(found, srv.URL+"/broken") {
		t.Errorf("no GX8003 for the broken external link:\n%s", found)
	}
	if strings.Contains(found, srv.URL+"/ok") {
		t.Errorf("GX8003 reported the good external link:\n%s", found)
	}
}
