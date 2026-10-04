# Toggle Group

A segmented control of exclusive choices.

## Usage

```gx
<togglegroup.ToggleGroup>
  <togglegroup.ToggleGroupItem name="align" value="left" checked={true}>Left</togglegroup.ToggleGroupItem>
  <togglegroup.ToggleGroupItem name="align" value="center">Center</togglegroup.ToggleGroupItem>
</togglegroup.ToggleGroup>
```

The group is a set of styled radio inputs, so it works without JavaScript.

## Do

- Give every item the same `Name`.
- Set `Checked` on exactly one item.

## Don't

- Do not use a toggle group for independent choices. Use toggles.
- Do not mix two groups under one name.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the checked item. |
| Arrow keys | Moves the choice and selects it. |
| Space | Selects the focused item. |
