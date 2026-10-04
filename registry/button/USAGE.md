# Button

A clickable control with variant and size options.

## Usage

```gx
<ui.Button>Save</ui.Button>
<ui.Button variant={ui.Destructive}>Delete</ui.Button>
<ui.Button variant={ui.Outline} size={ui.Sm}>Cancel</ui.Button>
```

Pass `Type` when the button submits a form: `<ui.Button type="submit">Send</ui.Button>`.

## Do

- Use one `default` button per view for the main action.
- Set `type="submit"` on a form button. The default `button` type does not submit.

## Don't

- Do not use a button as a link. Use `<ui.LinkButton>` (docs kit) or an `<a>` with a typed `href`.
- Do not put a button inside another button.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the button. |
| Enter | Activates the button. |
| Space | Activates the button. |
