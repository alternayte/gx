package starlight_test

import (
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

# Routing

<Tabs>
  <TabItem label="Postgres">Use Postgres.</TabItem>
  <TabItem label="SQL Server">Use SQL Server.</TabItem>
</Tabs>

<Steps>

1. Create the page.
2. Mount the route.

</Steps>

<Aside type="tip">Mount routes explicitly.</Aside>

<Chart data={[1, 2, 3]} />
`,
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
	for _, want := range []string{"order: 2", "sidebarGroup: Guides", "<docs.Tabs>", `<docs.TabItem label="Postgres">`, "<docs.Steps>", `<docs.Aside kind="tip">`} {
		if !strings.Contains(got, want) {
			t.Errorf("routing.md lacks %q:\n%s", want, got)
		}
	}
	for _, bad := range []string{"@astrojs/starlight", "import Chart", "<Tabs>", "<TabItem"} {
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
	joined := strings.Join(report.Notes, "\n")
	for _, want := range []string{"Chart", "collapsed", "auto-generates"} {
		if !strings.Contains(joined, want) {
			t.Errorf("report lacks %q:\n%s", want, joined)
		}
	}
}
