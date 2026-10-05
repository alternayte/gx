# Switch

A control for an immediate on or off setting.

## Usage

```gx
<switches.Switch name="wifi" label="Wifi" checked={p.Wifi} />
<switches.Switch name="sync" label="Sync" size={switches.Sm} />
```

The switch is a styled checkbox with the `switch` role.
The sizes are `switches.Md` and `switches.Sm`.
`Disabled` disables the checkbox.

## Do

- Use a switch when the change takes effect at once.
- Pair the switch with a visible label.

## Don't

- Do not use a switch inside a form that needs a submit button.
- Do not use a switch for a multi-choice setting.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the switch. |
| Space | Toggles the switch. |
