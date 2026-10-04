# Item

A row for one entry in a list.

## Usage

```gx
<item.Item class="p-4">
  <item.ItemMedia>*</item.ItemMedia>
  <item.ItemContent>
    <item.ItemTitle>Item title</item.ItemTitle>
    <item.ItemDescription>A short description.</item.ItemDescription>
  </item.ItemContent>
  <item.ItemActions><a href={items.Show{ID: it.ID}}>Open</a></item.ItemActions>
</item.Item>
```

## Do

- Keep one row per entry.
- Put the row action in `ItemActions`.

## Don't

- Do not put a whole table in an item.
- Do not nest one item in another.

## Keyboard

Rows take focus only when they contain a link or a control.
