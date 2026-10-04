# docs-shell

The Gx docs shell: header, sidebar, table of contents, pagination, splash,
404 and the search dialog. It pairs with the `docs` kit.

## Use

```go
var Site = shell.Config{Title: "Deedbox", Version: "v1.0.0", EditBase: "https://github.com/acme/site/edit/main/content/docs/"}

func View(e gx.Entry[DocMeta]) gx.Node {
    return shell.Shell(shell.ShellProps{Site: Site, Nav: Nav(), Page: PageFor(e), Children: DocsBody(e)})
}
```

Set `[site] url` in `gx.toml` so the export writes canonical links,
`sitemap.xml` and `robots.txt`.

## Do

- Sort the sidebar with explicit `order` values on the pages that need it.
- Keep the page `Path` with a trailing slash.

## Don't

- Do not render `Shell` without a `Page`; the table of contents and the
  navigation need the path.
- Do not put a `Toc` outside the shell.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Opens the search dialog or the mobile menu. |
| Ctrl/Cmd K | Opens the search dialog. |
| Escape | Closes the search dialog. |
| Tab | Moves through the header, the sidebar and the table of contents. |
