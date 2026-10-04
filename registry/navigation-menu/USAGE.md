# Navigation Menu

A row of primary links.

## Usage

```gx
<navigationmenu.NavigationMenu>
  <navigationmenu.NavigationMenuItem href={gx.URL("/")} active={true}>Home</navigationmenu.NavigationMenuItem>
  <navigationmenu.NavigationMenuItem href={gx.URL("/docs")}>Docs</navigationmenu.NavigationMenuItem>
</navigationmenu.NavigationMenu>
```

## Do

- Pass a typed route to `Href`.
- Mark the current page with `Active`.

## Don't

- Do not use it for a footer row of links.
- Do not mix it with a menubar on one row.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between the links. |
| Enter | Follows the focused link. |
