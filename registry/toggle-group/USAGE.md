# Toggle Group

A segmented control of toggle items.

## Usage

```gx
<togglegroup.ToggleGroup label="Alignment" variant={togglegroup.Outline}>
  <togglegroup.ToggleGroupItem name="align" value="left" checked={true}>Left</togglegroup.ToggleGroupItem>
  <togglegroup.ToggleGroupItem name="align" value="center">Center</togglegroup.ToggleGroupItem>
</togglegroup.ToggleGroup>
```

The group is a set of styled radio inputs, so it works without JavaScript.
`Variant` and `Size` on the group style every item.
`Spacing` is the gap between the items in spacing units. Zero joins the items and shares their borders.
Set `Multiple` on every item to make each item an independent checkbox.

## Do

- Give every item of an exclusive group the same `Name`.
- Set `Checked` on exactly one item of an exclusive group.
- Give the group a `Label`.

## Don't

- Do not mix exclusive items and `Multiple` items in one group.
- Do not mix two groups under one name.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the checked item. With `Multiple`, moves focus to each item. |
| Arrow keys | Moves the choice and selects it. |
| Space | Selects the focused item. With `Multiple`, toggles it. |
