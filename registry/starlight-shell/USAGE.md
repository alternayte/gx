# starlight-shell

The page structure of the default Starlight theme. It has these parts:

- A fixed header with the site title, a version label, the search button, social icons and a theme select.
- A sidebar and a table of contents.
- A hero for a splash page.
- Page links, a search dialog and a 404 page.

The block needs the `starlight` theme for its look and the `docs` kit for
its icons.

## Use

```go
var Site = starlight.Config{
    Title:    "Deedbox",
    Version:  "latest",
    Links:    []starlight.Link{{Label: "GitHub", Href: gx.URL("https://github.com/acme/site"), Icon: "github"}},
    EditBase: "https://github.com/acme/site/edit/main/content/docs/",
}

func View(e gx.Entry[DocMeta]) gx.Node {
    return starlight.Shell(starlight.ShellProps{Site: Site, Nav: Nav(), Page: PageFor(e), Children: DocsBody(e)})
}
```

A splash page has no sidebar and no table of contents. Give it a hero:

```go
starlight.Shell(starlight.ShellProps{
    Site: Site,
    Page: starlight.Page{Title: "Deedbox", Path: "/", Splash: true},
    Hero: starlight.Hero(starlight.HeroProps{
        Title:   "Deedbox",
        Tagline: "Event-source part of your app.",
        Actions: docs.LinkButton(docs.LinkButtonProps{Href: gx.URL("/start/"), Icon: "right-arrow", Children: gx.Text("Start")}),
    }),
    Children: DocsBody(e),
})
```

With two or more `Versions` in the config, the header shows a select. A
choice opens the site of that version.

## Do

- Keep the page `Path` with a trailing slash. The sidebar marks the item with the same path.
- Name a built-in icon of the docs kit for each social link, for example `github`.
- Set `Collapsed` on a long nested group. The group opens when it holds the current page.

## Don't

- Do not use the block without the `starlight` theme. The markup has no utility classes.
- Do not put a `Toc` or a `Sidebar` outside the `Shell`.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves through the skip link, the header, the sidebar, the content and the table of contents. |
| Enter, Space | Opens the search dialog, the mobile menu, a sidebar group or the table of contents menu. |
| Ctrl/Cmd K | Opens the search dialog. |
| Escape | Closes the search dialog and the table of contents menu. |
| Arrow up, Arrow down | Changes the theme or the version in its select. |
