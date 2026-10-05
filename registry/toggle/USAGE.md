# Toggle

A two-state button.

## Usage

```gx
<toggle.Toggle name="bold" pressed={p.Bold}>Bold</toggle.Toggle>
<toggle.Toggle name="italic" variant={toggle.Outline} size={toggle.Sm}>Italic</toggle.Toggle>
```

The toggle is a styled checkbox, so it works without JavaScript.
The variants are `toggle.Default` and `toggle.Outline`. The sizes are `toggle.Sm`, `toggle.Md` and `toggle.Lg`.
`Disabled` and `Invalid` set the state of the checkbox.

## Do

- Give the toggle a name that reads as a state.
- Use it for a formatting choice.

## Don't

- Do not use a toggle for an immediate setting. Use a switch.
- Do not use a toggle for a group of choices. Use a toggle group.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the toggle. |
| Space | Toggles the state. |
