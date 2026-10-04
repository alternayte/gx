# Login

A sign-in page block with a form and a brand panel.

## Usage

```gx
<login.Login action="/signin" />
```

The block is copied source. Replace the raw form with a typed `gx.Form` and route input when the app has its sign-in route.

## Do

- Keep the form to two fields.
- Link to the sign-up page.

## Don't

- Do not show a password hint on the page.
- Do not remove the brand panel on wide screens.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between the email field, the password field and the button. |
| Enter | Submits the form from a field. |
