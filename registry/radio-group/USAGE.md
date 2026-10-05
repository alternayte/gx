# Radio Group

A group of exclusive choices.

## Usage

```gx
<radiogroup.RadioGroup label="Plan">
  <radiogroup.RadioGroupItem name="plan" value="free" checked={true} label="Free" />
  <radiogroup.RadioGroupItem name="plan" value="pro" label="Pro" />
</radiogroup.RadioGroup>
```

Each item is a native radio input behind a styled circle.
`Disabled` and `Invalid` set the state of the input.

## Do

- Give every item the same `Name`.
- Set `Checked` on exactly one item.
- Give the group a `Label`.

## Don't

- Do not use a radio group for a boolean. Use a checkbox or a switch.
- Do not mix two names in one group.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the checked item. |
| Arrow keys | Moves the choice and selects it. |
| Space | Selects the focused item. |
