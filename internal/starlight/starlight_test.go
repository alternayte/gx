package starlight_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/starlight"
)

// TestREQ_CNT_13_StarlightImport covers `gx import starlight`: MDX to
// Markdown, Starlight component tags to the docs kit, frontmatter from the
// sidebar config and a report of what it could not convert (REQ-CNT-13).
func TestREQ_CNT_13_StarlightImport(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"astro.config.mjs": `import { defineConfig } from 'astro/config'
import starlight from '@astrojs/starlight'

export default defineConfig({
  integrations: [
    starlight({
      title: 'Deedbox',
      sidebar: [
        { label: 'Guides', items: ['guides/routing', { slug: 'guides/pages', badge: 'New' }] },
        { label: 'Errors', collapsed: true, autogenerate: { directory: 'errors' } },
      ],
    }),
  ],
})
`,
		"src/content/docs/guides/routing.mdx": `---
title: Routing
description: Routes are Go types.
sidebar:
  order: 2
---

import { Tabs, TabItem, Steps, Aside } from '@astrojs/starlight/components';
import Chart from '../../components/Chart.astro';
import CartEvents from '../../../snippets/cart-events.md';

# Routing

<Tabs syncKey="db">
  <TabItem label="Postgres">Use Postgres.</TabItem>
  <TabItem label="SQL Server">Use SQL Server.</TabItem>
</Tabs>

<Steps>

1. Create the page.
2. Mount the route.

</Steps>

<Aside type="tip">Mount routes explicitly.</Aside>

<CartEvents />

~~~cs
EventBuilder<TEvent>
~~~

<Chart data={[1, 2, 3]} />
`,
		"src/snippets/cart-events.md": "<!-- snippet: cart-events -->\n```cs\npublic record ItemAdded(string Sku, int Qty);\n```\n<!-- endSnippet -->\n",

		"src/content/docs/guides/pages.md":  "---\ntitle: Pages\n---\n\n# Pages\n",
		"src/content/docs/errors/GX1000.md": "---\ntitle: GX1000\n---\n\n# GX1000\n",
	}
	for rel, body := range files {
		path := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	report, err := starlight.Convert(starlight.Options{Src: src, Out: out})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if report.Converted != 3 {
		t.Fatalf("Converted = %d, want 3 (%v)", report.Converted, report.Notes)
	}
	routing, err := os.ReadFile(filepath.Join(out, "guides", "routing.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(routing)
	for _, want := range []string{"order: 2", "sidebarGroup: Guides", "<docs.Tabs sync=\"db\">", `<docs.TabItem label="Postgres">`, "<docs.Steps>", `<docs.Aside kind="tip">`, "public record ItemAdded", "EventBuilder<TEvent>"} {
		if !strings.Contains(got, want) {
			t.Errorf("routing.md lacks %q:\n%s", want, got)
		}
	}
	for _, bad := range []string{"@astrojs/starlight", "import Chart", "<Tabs>", "<TabItem", "<CartEvents"} {
		if strings.Contains(got, bad) {
			t.Errorf("routing.md kept %q:\n%s", bad, got)
		}
	}
	pages, err := os.ReadFile(filepath.Join(out, "guides", "pages.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pages), "sidebarGroup: Guides") || !strings.Contains(string(pages), "badge: New") {
		t.Errorf("pages.md = %s", pages)
	}
	errors, err := os.ReadFile(filepath.Join(out, "errors", "GX1000.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(errors), "sidebarGroup: Errors") || !strings.Contains(string(errors), "collapsed: true") {
		t.Errorf("GX1000.md = %s", errors)
	}
	joined := strings.Join(report.Notes, "\n")
	for _, want := range []string{"Chart", "auto-generates"} {
		if !strings.Contains(joined, want) {
			t.Errorf("report lacks %q:\n%s", want, joined)
		}
	}
}

// tarGz builds one gzip tarball from a path-to-body map.
func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestREQ_CNT_13_RemoteImport covers the GitHub import: the repository
// archive downloads, the Starlight project is found and the content is
// converted (REQ-CNT-13).
func TestREQ_CNT_13_RemoteImport(t *testing.T) {
	archive := tarGz(t, map[string]string{
		"docs-main/README.md":                      "# Docs\n",
		"docs-main/site/astro.config.mjs":          "import starlight from '@astrojs/starlight'\nexport default defineConfig({ integrations: [starlight({ title: 'Docs', sidebar: [{ label: 'Start', items: ['start'] }] })] })\n",
		"docs-main/site/src/content/docs/start.md": "---\ntitle: Start\n---\n\n# Start\n",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/acme/docs/tar.gz/main" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(archive)
	}))
	defer srv.Close()

	out := t.TempDir()
	report, err := starlight.Convert(starlight.Options{
		Src:        "https://github.com/acme/docs",
		Out:        out,
		GitHubBase: srv.URL,
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if report.Converted != 1 {
		t.Fatalf("Converted = %d, notes = %v", report.Converted, report.Notes)
	}
	page, err := os.ReadFile(filepath.Join(out, "start.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"title: Start", "sidebarGroup: Start"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("start.md lacks %q:\n%s", want, page)
		}
	}
}
