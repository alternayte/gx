# Signup

A sign-up page block with fields, terms and a brand panel.

## Usage

```gx
<signup.Signup action="/signup" />
```

The block is copied source. Replace the raw form with a typed `gx.Form` and route input when the app has its sign-up route.

## Do

- State the password rule in the hint.
- Keep the terms checkbox next to the submit button.

## Don't

- Do not ask for data the app does not need.
- Do not submit before the terms are accepted.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between fields, the terms checkbox and the button. |
| Space | Toggles the terms checkbox. |
| Enter | Submits the form from a field. |
