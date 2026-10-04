# Input Group

An input with an addon or a text prefix.

## Usage

```gx
<inputgroup.InputGroup>
  <inputgroup.InputGroupAddon>
    <inputgroup.InputGroupText>$</inputgroup.InputGroupText>
  </inputgroup.InputGroupAddon>
  <inputgroup.InputGroupInput placeholder="0.00" />
</inputgroup.InputGroup>
```

## Do

- Put short text or an icon in the addon.
- Use `Align` to move the addon to the end.

## Don't

- Do not put a label inside the addon. Use a `label.Label` above the group.
- Do not put more than one input in a group.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the input or the addon button. |
| Enter | Submits the form when the input is in a form. |
