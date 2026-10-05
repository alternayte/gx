# Input Group

An input or a textarea with text, icons or buttons inside its border.

## Usage

```gx
<inputgroup.InputGroup>
  <inputgroup.InputGroupAddon><icons.Info /></inputgroup.InputGroupAddon>
  <inputgroup.InputGroupInput name="q" placeholder="Search" />
  <inputgroup.InputGroupAddon align={inputgroup.InlineEnd}>
    <inputgroup.InputGroupButton>Search</inputgroup.InputGroupButton>
  </inputgroup.InputGroupAddon>
</inputgroup.InputGroup>

<inputgroup.InputGroup>
  <inputgroup.InputGroupTextarea name="message" placeholder="Ask a question." />
  <inputgroup.InputGroupAddon align={inputgroup.BlockEnd}>
    <inputgroup.InputGroupText>120 characters left</inputgroup.InputGroupText>
  </inputgroup.InputGroupAddon>
</inputgroup.InputGroup>
```

An addon has four positions: `InlineStart`, `InlineEnd`, `BlockStart` and `BlockEnd`. A block addon makes the group a column.
`InputGroupButton` is a ghost button with the sizes `Xs`, `Sm`, `IconXs` and `IconSm`.
The group shows the focus ring of its control. It shows the error border when the control has `aria-invalid="true"`.
A click on an addon does not focus the control. Put the addon text in a `<label for>` when a click must focus it.

## Do

- Give the control an `aria-label` or a label. An addon is not a label.
- Put a block addon with a textarea.

## Don't

- Do not put more than one control in a group.
- Do not use an addon for an error message. Use a field error.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the control, then to each button of the group. |
| Enter, Space | Activates the focused button. |
