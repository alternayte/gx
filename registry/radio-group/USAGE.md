# Radio Group

A group of exclusive choices.

## Usage

```gx
<radiogroup.RadioGroup name="plan">
  <radiogroup.RadioGroupItem name="plan" value="free" checked={true} label="Free" />
  <radiogroup.RadioGroupItem name="plan" value="pro" label="Pro" />
</radiogroup.RadioGroup>
```

## Do

- Give every item the same `Name`.
- Set `Checked` on exactly one item.

## Don't

- Do not use a radio group for a boolean. Use a checkbox or a switch.
- Do not mix two names in one group.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the checked item. |
| Arrow keys | Moves the choice and selects it. |
| Space | Selects the focused item. |
