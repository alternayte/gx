# Navigation Menu

A row of primary links and menus of links.

## Usage

```gx
<navigationmenu.NavigationMenu label="Main">
  <navigationmenu.NavigationMenuItem>
    <navigationmenu.NavigationMenuTrigger>Products</navigationmenu.NavigationMenuTrigger>
    <navigationmenu.NavigationMenuContent>
      <ul class="grid w-48 gap-1">
        <li><navigationmenu.NavigationMenuLink href={gx.URL("/products")}>All products</navigationmenu.NavigationMenuLink></li>
        <li><navigationmenu.NavigationMenuLink href={gx.URL("/products/new")}>New arrivals</navigationmenu.NavigationMenuLink></li>
      </ul>
    </navigationmenu.NavigationMenuContent>
  </navigationmenu.NavigationMenuItem>
  <navigationmenu.NavigationMenuItem>
    <navigationmenu.NavigationMenuLink href={gx.URL("/docs")} variant={navigationmenu.Trigger} active={true}>Docs</navigationmenu.NavigationMenuLink>
  </navigationmenu.NavigationMenuItem>
</navigationmenu.NavigationMenu>
```

`NavigationMenu` renders the `nav` and its list. Each `NavigationMenuItem` holds one link, or one trigger with its content. A link in the bar takes `variant={navigationmenu.Trigger}`, the style of a trigger.

The content is CSS only. It shows when the pointer is on the item and when focus is in the item, below its own item. It has no shared viewport and no indicator.

## Do

- Pass a typed route to `Href`.
- Mark the current page with `Active`.
- Give the menu a `Label` when a page has more than one `nav`.

## Don't

- Do not use it for a footer row of links.
- Do not put an action in the content. Use a dropdown menu.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between the links and triggers. The content of a focused trigger shows. |
| Tab | From a trigger, moves into its content. The content closes when focus leaves the item. |
| Enter | Follows the focused link. |
