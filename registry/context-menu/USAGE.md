# Context Menu

A menu that opens on a right click.

## Usage

```gx
<contextmenu.ContextMenuTrigger id="row-menu">Right-click here</contextmenu.ContextMenuTrigger>
<contextmenu.ContextMenu id="row-menu">
  <contextmenu.ContextMenuItem>Copy</contextmenu.ContextMenuItem>
  <contextmenu.ContextMenuSeparator />
  <contextmenu.ContextMenuLink href={gx.URL("/docs")}>Docs</contextmenu.ContextMenuLink>
</contextmenu.ContextMenu>
```

## Do

- Attach the menu to the object it acts on.
- Keep the same items as the visible row menu.

## Don't

- Do not hide the only action behind a right click.
- Do not open a context menu on a whole page.

## Keyboard

| Key | Action |
| --- | --- |
| Context menu key | Opens the menu at the focused item. |
| Arrow keys | Moves through the items. |
| Escape | Closes the menu. |
