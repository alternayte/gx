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

`MenubarMenu` renders the trigger in the bar and the menu that opens from it. `Label` is the text of the trigger and `Id` is the id of the menu. The menu uses the native Popover API. A browser without anchor positioning shows the menu at its place in the document flow.

A checkbox item and a radio item are native inputs. They carry `Name` and `Value`, so a form or a signal reads them. They keep the menu open. An item has `Inset`, `Disabled` and the `Destructive` variant.

A menu fades and zooms in and out. Safari shows the enter transition only. A user who asks for reduced motion gets no transition.

A click opens a menu. The pointer does not open the next menu when it moves along the bar. The item has no sub-menu part.

## Do

- Give each menu a unique `Id`.
- Keep the bar to one row.
- Give the radio items of one group the same `Name`.

## Don't

- Do not put a link or a button directly in the bar. Put it in a menu.
- Do not use a menubar for a site header. Use the navigation menu.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Enters the bar at one trigger and leaves it again. |
| Left, Right | On a trigger, moves to the previous or next trigger. In a menu, opens the previous or next menu. |
| Home, End | Moves to the first or last trigger, or to the first or last item of a menu. |
| Enter, Space | Opens the menu of the focused trigger. |
| Down | Opens the menu and moves to the first item. |
| Up, Down | Moves through the items. A disabled item is passed. |
| A letter | Moves to the next item that starts with the typed text. |
| Enter | Runs the focused item and closes the menu. Toggles a checkbox or radio item. |
| Escape | Closes the menu and returns focus to its trigger. |
