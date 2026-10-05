# Button

A clickable control with variant and size options.

## Usage

```gx
<button.Button>Save</button.Button>
<button.Button variant={button.Destructive}>Delete</button.Button>
<button.Button variant={button.Outline} size={button.Sm}>Cancel</button.Button>
<button.Button variant={button.Ghost} size={button.Icon} attrs={gx.Attrs{{Key: "aria-label", Value: "Close"}}}>
  <icons.X />
</button.Button>
```

Pass `Type` when the button submits a form: `<button.Button type="submit">Send</button.Button>`.

The sizes are `Xs`, `Sm`, `Md`, `Lg`, `Icon`, `IconXs`, `IconSm` and `IconLg`.
An icon without a `size-` class takes the size of the button.

A link that looks like a button takes `button.Class` on its anchor: `<a href={route} class={button.Class(button.Outline, button.Sm)}>Docs</a>`.

## Do

- Use one `default` button per view for the main action.
- Set `type="submit"` on a form button. The default `button` type does not submit.
- Give an icon-only button an `aria-label`.

## Don't

- Do not use a button as a link. Use an `<a>` with a typed `href` and `button.Class`.
- Do not put a button inside another button.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the button. |
| Enter | Activates the button. |
| Space | Activates the button. |
