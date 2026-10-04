# Dropdown Menu

A menu of actions behind a trigger.

## Usage

```gx
<dropdownmenu.DropdownMenuTrigger id="account">Account</dropdownmenu.DropdownMenuTrigger>
<dropdownmenu.DropdownMenu id="account">
  <dropdownmenu.DropdownMenuLabel>My account</dropdownmenu.DropdownMenuLabel>
  <dropdownmenu.DropdownMenuSeparator />
  <dropdownmenu.DropdownMenuLink href={gx.URL("/profile")}>Profile</dropdownmenu.DropdownMenuLink>
  <dropdownmenu.DropdownMenuItem>Sign out</dropdownmenu.DropdownMenuItem>
</dropdownmenu.DropdownMenu>
```

## Do

- Keep one word or one short phrase per item.
- Put destructive actions last, after a separator.

## Don't

- Do not use a dropdown menu as a select control. Use `select`.
- Do not nest a dropdown menu in another dropdown menu.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Opens the menu. |
| Arrow keys | Moves through the items. |
| Enter | Runs the focused item. |
| Escape | Closes the menu. |
