# Select

A native list of exclusive choices.

## Usage

```gx
<select.Select name="plan">
  <select.SelectOption value="free" selected={true}>Free</select.SelectOption>
  <select.SelectOption value="pro">Pro</select.SelectOption>
</select.Select>
```

The component is a styled native `<select>`, so the keyboard and the mobile picker come from the browser.

## Do

- Mark one option with `Selected`.
- Pair the select with a label.

## Don't

- Do not use a select for fewer than four options. Use a radio group.
- Do not leave the select without a name in a form.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the select. |
| Arrow keys | Moves through the options. |
| Enter, Space | Opens or commits the choice. |
