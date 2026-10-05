# Sidebar

A vertical panel for the app or docs navigation.

## Usage

```gx
<sidebar.Sidebar id="gx-sidebar">
  <sidebar.SidebarHeader>
    <sidebar.SidebarInput name="q" placeholder="Search" />
  </sidebar.SidebarHeader>
  <sidebar.SidebarContent>
    <sidebar.SidebarGroup>
      <sidebar.SidebarGroupLabel>Menu</sidebar.SidebarGroupLabel>
      <sidebar.SidebarGroupContent>
        <sidebar.SidebarMenu>
          <sidebar.SidebarMenuItem>
            <sidebar.SidebarMenuButton href={homeRoute} active={true}><span>Home</span></sidebar.SidebarMenuButton>
          </sidebar.SidebarMenuItem>
          <sidebar.SidebarMenuItem>
            <sidebar.SidebarMenuButton href={docsRoute}><span>Docs</span></sidebar.SidebarMenuButton>
            <sidebar.SidebarMenuBadge>12</sidebar.SidebarMenuBadge>
            <sidebar.SidebarMenuSub>
              <sidebar.SidebarMenuSubItem>
                <sidebar.SidebarMenuSubButton href={formsRoute}><span>Forms</span></sidebar.SidebarMenuSubButton>
              </sidebar.SidebarMenuSubItem>
            </sidebar.SidebarMenuSub>
          </sidebar.SidebarMenuItem>
        </sidebar.SidebarMenu>
      </sidebar.SidebarGroupContent>
    </sidebar.SidebarGroup>
  </sidebar.SidebarContent>
  <sidebar.SidebarFooter>v0.1.0</sidebar.SidebarFooter>
</sidebar.Sidebar>
<sidebar.SidebarInset>
  <sidebar.SidebarTrigger class="lg:hidden" />
</sidebar.SidebarInset>
```

`SidebarMenuButton` is a link when it has `Href`, and a button when it has none. Its variants are `Default` and `Outline`. Its sizes are `Md`, `Sm` and `Lg`.
`SidebarMenuAction` and `SidebarMenuBadge` sit at the right of the button of their item. `SidebarGroupAction` sits at the right of the group label.
`SidebarMenuSkeleton` is the placeholder of one menu row.
`SidebarTrigger` shows and hides the sidebar with the id `gx-sidebar` on a narrow screen. The sidebar is always visible on a wide screen.
Set `side={sidebar.Right}` for a sidebar at the right edge.

The sidebar is static. It has no collapsed icon mode, no rail, no floating or inset variant and no mobile sheet.

## Do

- Give the sidebar the id `gx-sidebar` when it pairs with `SidebarTrigger`.
- Put the text of a menu button in a `<span>`. A long text is then cut with an ellipsis.
- Give `SidebarMenuAction` and `SidebarGroupAction` a `Label`.

## Don't

- Do not put a second sidebar on one page.
- Do not hide the sidebar on a wide screen.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between the links and the buttons. |
| Enter | Follows the focused link, or activates the focused button. |
| Space | Activates the focused button. |
