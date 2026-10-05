# Item

A row with media, a title, a description and actions.

## Usage

```gx
<item.Item variant={item.Outline}>
  <item.ItemMedia variant={item.MediaIcon}><icons.Info /></item.ItemMedia>
  <item.ItemContent>
    <item.ItemTitle>Item title</item.ItemTitle>
    <item.ItemDescription>A short description of the item.</item.ItemDescription>
  </item.ItemContent>
  <item.ItemActions>
    <button.Button variant={button.Outline} size={button.Sm}>Open</button.Button>
  </item.ItemActions>
</item.Item>

<item.ItemGroup>
  <item.Item>...</item.Item>
  <item.ItemSeparator />
  <item.Item>...</item.Item>
</item.ItemGroup>
```

The variants are `Default`, `Outline` and `Muted`. The sizes are `Md` and `Sm`.
`ItemMedia` has the variants `MediaDefault`, `MediaIcon` and `MediaImage`. The media aligns with the title when the item has a description.

Set `Href` to make the whole item one link. A link item has a hover state.
`ItemHeader` and `ItemFooter` take a full row above and below the content.

## Do

- Keep the title to one line.
- Use `ItemGroup` and `ItemSeparator` for a list of items.

## Don't

- Do not put a button in an item that has `Href`. A link does not hold a button.
- Do not use an item for tabular data. Use a table.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to a link item, then to the controls in `ItemActions`. |
| Enter | Follows a link item. |
