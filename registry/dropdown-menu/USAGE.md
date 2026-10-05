# Dropdown Menu

A menu of actions behind a trigger.

## Usage

```gx
<dropdownmenu.DropdownMenuTrigger id="account">Account</dropdownmenu.DropdownMenuTrigger>
<dropdownmenu.DropdownMenu id="account" class="w-56">
  <dropdownmenu.DropdownMenuLabel>My account</dropdownmenu.DropdownMenuLabel>
  <dropdownmenu.DropdownMenuSeparator />
  <dropdownmenu.DropdownMenuGroup label="Account">
    <dropdownmenu.DropdownMenuLink href={gx.URL("/profile")}>Profile</dropdownmenu.DropdownMenuLink>
    <dropdownmenu.DropdownMenuItem>
      Settings
      <dropdownmenu.DropdownMenuShortcut>⌘S</dropdownmenu.DropdownMenuShortcut>
    </dropdownmenu.DropdownMenuItem>
  </dropdownmenu.DropdownMenuGroup>
  <dropdownmenu.DropdownMenuSeparator />
  <dropdownmenu.DropdownMenuCheckboxItem name="status-bar" checked={true}>Status bar</dropdownmenu.DropdownMenuCheckboxItem>
  <dropdownmenu.DropdownMenuRadioGroup label="Position">
    <dropdownmenu.DropdownMenuRadioItem name="position" value="top" checked={true}>Top</dropdownmenu.DropdownMenuRadioItem>
    <dropdownmenu.DropdownMenuRadioItem name="position" value="bottom">Bottom</dropdownmenu.DropdownMenuRadioItem>
  </dropdownmenu.DropdownMenuRadioGroup>
  <dropdownmenu.DropdownMenuSeparator />
  <dropdownmenu.DropdownMenuItem variant={dropdownmenu.Destructive}>Sign out</dropdownmenu.DropdownMenuItem>
</dropdownmenu.DropdownMenu>
```

The menu uses the native Popover API. The trigger is a `button.Button`; `Variant` and `Size` select its style, and the default is `button.Outline`. `Align` lines the menu up with the trigger: `dropdownmenu.Center` (default), `dropdownmenu.Start` or `dropdownmenu.End`. A browser without anchor positioning shows the menu at its place in the document flow.

A checkbox item and a radio item are native inputs. They carry `Name` and `Value`, so a form or a signal reads them. They keep the menu open. An item has `Inset`, `Disabled` and the `Destructive` variant.

The menu fades and zooms in and out. Safari shows the enter transition only. A user who asks for reduced motion gets no transition.

The item has no sub-menu part.

## Do

- Give the menu and the trigger the same `Id`.
- Put the menu directly after its trigger in the markup.
- Give the radio items of one group the same `Name`.
- Put destructive actions last, after a separator.

## Don't

- Do not use a dropdown menu as a select control. Use `select`.
- Do not nest a dropdown menu in another dropdown menu.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Opens the menu from the trigger. |
| Down | Opens the menu from the trigger and moves to the first item. |
| Up, Down | Moves through the items. A disabled item is passed. |
| Home, End | Moves to the first or last item. |
| A letter | Moves to the next item that starts with the typed text. |
| Enter | Runs the focused item and closes the menu. Toggles a checkbox or radio item. |
| Space | Runs the focused item. Toggles a checkbox or radio item. |
| Escape | Closes the menu and returns focus to the trigger. |
