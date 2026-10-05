# Checkbox

A single boolean control with a label.

## Usage

```gx
<checkbox.Checkbox name="terms" checked={p.Accepted}>Accept the terms</checkbox.Checkbox>
<checkbox.Checkbox name="terms" invalid={p.TermsMissing}>Accept the terms</checkbox.Checkbox>
```

The checkbox is a native input behind a styled box.
`Disabled` and `Invalid` set the state of the input.

## Do

- Use one checkbox per independent choice.
- Wrap the control and its label in the component.

## Don't

- Do not use a checkbox for a mutually exclusive choice. Use a radio group.
- Do not use a checkbox for an immediate action. Use a switch.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the checkbox. |
| Space | Toggles the value. |
