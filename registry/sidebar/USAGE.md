# Sidebar

A vertical panel for the app or docs navigation.

## Usage

```gx
<sidebar.Sidebar id="gx-sidebar">
  <sidebar.SidebarHeader>Gx</sidebar.SidebarHeader>
  <sidebar.SidebarContent>
    <sidebar.SidebarGroup title="Menu">
      <sidebar.SidebarItem href={gx.URL("/")} active={true}>Home</sidebar.SidebarItem>
    </sidebar.SidebarGroup>
  </sidebar.SidebarContent>
</sidebar.Sidebar>
```

Pair the sidebar with a button that carries `data-gx-menu` and `aria-controls` set to the sidebar id.

## Do

- Give the sidebar the id `gx-sidebar` when it pairs with the shell menu button.
- Keep groups short.

## Don't

- Do not put a second sidebar on one page.
- Do not hide the sidebar on a wide screen.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between the items. |
| Enter | Follows the focused link. |
