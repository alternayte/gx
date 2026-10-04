# Input

A single-line text control.

## Usage

```gx
<input.Input name="email" type="email" placeholder="Email" />
```

For a form with rules and errors, use the generated `gx.FormField` control instead.

## Do

- Always pair an input with a label.
- Set `Type` to the data the field expects.

## Don't

- Do not use placeholder text as a label.
- Do not build `value` from untrusted text without escaping. Gx escapes attribute values.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the input. |
| Enter | Submits the form when the input is in a form. |
