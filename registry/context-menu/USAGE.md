# Context Menu

A menu that opens on a right click.

## Usage

```gx
<contextmenu.ContextMenuTrigger id="row-menu" class="rounded-md border border-dashed p-8">Right-click here</contextmenu.ContextMenuTrigger>
<contextmenu.ContextMenu id="row-menu" class="w-52">
  <contextmenu.ContextMenuItem>
    Copy
    <contextmenu.ContextMenuShortcut>⌘C</contextmenu.ContextMenuShortcut>
  </contextmenu.ContextMenuItem>
  <contextmenu.ContextMenuSeparator />
  <contextmenu.ContextMenuCheckboxItem name="bookmarks" checked={true}>Show bookmarks</contextmenu.ContextMenuCheckboxItem>
  <contextmenu.ContextMenuRadioGroup label="People">
    <contextmenu.ContextMenuLabel inset={true}>People</contextmenu.ContextMenuLabel>
    <contextmenu.ContextMenuRadioItem name="person" value="ada" checked={true}>Ada</contextmenu.ContextMenuRadioItem>
    <contextmenu.ContextMenuRadioItem name="person" value="grace">Grace</contextmenu.ContextMenuRadioItem>
  </contextmenu.ContextMenuRadioGroup>
  <contextmenu.ContextMenuSeparator />
  <contextmenu.ContextMenuSub>
    <contextmenu.ContextMenuSubTrigger>More tools</contextmenu.ContextMenuSubTrigger>
    <contextmenu.ContextMenuSubContent>
      <contextmenu.ContextMenuItem>Save page</contextmenu.ContextMenuItem>
      <contextmenu.ContextMenuItem>Developer tools</contextmenu.ContextMenuItem>
    </contextmenu.ContextMenuSubContent>
  </contextmenu.ContextMenuSub>
  <contextmenu.ContextMenuLink href={gx.URL("/docs")}>Docs</contextmenu.ContextMenuLink>
  <contextmenu.ContextMenuItem variant={contextmenu.Destructive}>Delete</contextmenu.ContextMenuItem>
</contextmenu.ContextMenu>
```

The trigger is an area with no style of its own; `Class` gives it one. The menu opens at the pointer and focus moves to the first item. It opens to the left of the pointer when the right side is too small, and it shifts up to stay 8px inside the viewport. `data-side` on the menu names the side it took.

A checkbox item and a radio item are native inputs. They carry `Name` and `Value`, so a form or a signal reads them. They keep the menu open. An item has `Inset`, `Disabled` and the `Destructive` variant.

The menu fades and zooms in and out. A user who asks for reduced motion gets no transition.

`ContextMenuSub` holds one `ContextMenuSubTrigger` and one `ContextMenuSubContent`, in that order. The sub-menu opens to the right of its trigger, or to the left when the right side is too small. It opens on a click and after the pointer rests on the trigger. It stays open while the pointer moves toward it. An item of a sub-menu closes the whole menu. The trigger has `Inset` and `Disabled`.

## Do

- Attach the menu to the object it acts on.
- Keep the same items as the visible row menu.
- Give the radio items of one group the same `Name`.

## Don't

- Do not hide the only action behind a right click.
- Do not open a context menu on a whole page.
- Do not put an element between the trigger and the content of a sub-menu.

## Keyboard

| Key | Action |
| --- | --- |
| Context menu key, Shift+F10 | Opens the menu at the focused element of the area. |
| Up, Down | Moves through the items. A disabled item is passed. |
| Home, End | Moves to the first or last item. |
| A letter | Moves to the next item that starts with the typed text. |
| Enter | Runs the focused item and closes the menu. Toggles a checkbox or radio item. |
| Space | Runs the focused item. Toggles a checkbox or radio item. |
| Right, Enter, Space | On a sub-menu trigger, opens the sub-menu and moves to its first item. |
| Left, Escape | In a sub-menu, closes it and returns focus to its trigger. |
| Escape | Closes the menu. |
