# Menubar

A horizontal bar of menu actions.

## Usage

```gx
<menubar.Menubar>
  <menubar.MenubarItem href={gx.URL("/")} active={true}>Home</menubar.MenubarItem>
  <menubar.MenubarItem href={gx.URL("/docs")}>Docs</menubar.MenubarItem>
</menubar.Menubar>
```

## Do

- Keep the bar to one row.
- Mark the current page with `Active`.

## Don't

- Do not mix unrelated destinations in one menubar.
- Do not use a menubar for a site header. Use the navigation menu.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Enters the bar at the first item. |
| Left, Right | Moves focus between items. |
| Home, End | Moves to the first or last item. |
| Enter | Follows the focused link. |
