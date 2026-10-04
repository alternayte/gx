# Settings Form

A settings page block with profile and notification cards.

## Usage

```gx
<settingsform.SettingsForm action="/settings" />
```

The block is copied source. Replace the raw form with a typed `gx.Form` when the app has rules for the fields.

## Do

- Group related settings in one card.
- Keep the save button after the last card.

## Don't

- Do not save each switch on its own.
- Do not put more than three cards on one page.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between fields, switches and the save button. |
| Space | Toggles a switch. |
| Enter | Submits the form from a field. |
