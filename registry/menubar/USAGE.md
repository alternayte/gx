# Menubar

A horizontal bar of menus.

## Usage

```gx
<menubar.Menubar>
  <menubar.MenubarMenu id="file-menu" label="File">
    <menubar.MenubarItem>
      New tab
      <menubar.MenubarShortcut>⌘T</menubar.MenubarShortcut>
    </menubar.MenubarItem>
    <menubar.MenubarSub>
      <menubar.MenubarSubTrigger>Share</menubar.MenubarSubTrigger>
      <menubar.MenubarSubContent>
        <menubar.MenubarItem>Email link</menubar.MenubarItem>
        <menubar.MenubarItem>Messages</menubar.MenubarItem>
      </menubar.MenubarSubContent>
    </menubar.MenubarSub>
    <menubar.MenubarSeparator />
    <menubar.MenubarLink href={gx.URL("/docs")}>Documentation</menubar.MenubarLink>
  </menubar.MenubarMenu>
  <menubar.MenubarMenu id="view-menu" label="View">
    <menubar.MenubarCheckboxItem name="urls" checked={true}>Always show full URLs</menubar.MenubarCheckboxItem>
    <menubar.MenubarSeparator />
    <menubar.MenubarItem inset={true}>Reload</menubar.MenubarItem>
  </menubar.MenubarMenu>
</menubar.Menubar>
```

`MenubarMenu` renders the trigger in the bar and the menu that opens from it. `Label` is the text of the trigger and `Id` is the id of the menu. The menu uses the native Popover API.

A menu opens below its trigger. It flips above the trigger when the space below is too small, and it shifts along the trigger to stay 8px inside the viewport. `data-side` on the menu names the side it took.

A checkbox item and a radio item are native inputs. They carry `Name` and `Value`, so a form or a signal reads them. They keep the menu open. An item has `Inset`, `Disabled` and the `Destructive` variant.

A menu fades and zooms in and out. Safari shows the enter transition only. A user who asks for reduced motion gets no transition.

A click opens a menu. With one menu open, the pointer opens the menu of each trigger it moves over.

`MenubarSub` holds one `MenubarSubTrigger` and one `MenubarSubContent`, in that order. The sub-menu opens to the right of its trigger, or to the left when the right side is too small. It opens on a click and after the pointer rests on the trigger. It stays open while the pointer moves toward it. An item of a sub-menu closes the whole menu. The trigger has `Inset` and `Disabled`.

## Do

- Give each menu a unique `Id`.
- Keep the bar to one row.
- Give the radio items of one group the same `Name`.

## Don't

- Do not put a link or a button directly in the bar. Put it in a menu.
- Do not use a menubar for a site header. Use the navigation menu.
- Do not put an element between the trigger and the content of a sub-menu.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Enters the bar at one trigger and leaves it again. |
| Left, Right | On a trigger, moves to the previous or next trigger. In a menu, opens the previous or next menu. |
| Right, Enter, Space | On a sub-menu trigger, opens the sub-menu and moves to its first item. |
| Left, Escape | In a sub-menu, closes it and returns focus to its trigger. |
| Home, End | Moves to the first or last trigger, or to the first or last item of a menu. |
| Enter, Space | Opens the menu of the focused trigger. |
| Down | Opens the menu and moves to the first item. |
| Up, Down | Moves through the items. A disabled item is passed. |
| A letter | Moves to the next item that starts with the typed text. |
| Enter | Runs the focused item and closes the menu. Toggles a checkbox or radio item. |
| Escape | Closes the menu and returns focus to its trigger. |
