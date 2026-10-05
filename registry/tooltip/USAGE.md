# Tooltip

A short hint on hover or focus.

## Usage

```gx
<tooltip.Tooltip content="Add to library">
  <button.Button variant={button.Outline}>Add</button.Button>
</tooltip.Tooltip>
```

The tooltip is CSS only. It shows on hover and on keyboard focus, with an arrow that points at the control. `Side` selects the edge: `tooltip.Top` (default), `tooltip.Bottom`, `tooltip.Left` or `tooltip.Right`.

The tooltip fades, zooms and slides in from the control. A user who asks for reduced motion gets no transition.

## Do

- Keep the text to a few words.
- Wrap a control that needs a name.

## Don't

- Do not put essential information only in a tooltip.
- Do not put a link inside a tooltip.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | The tooltip appears when the control takes keyboard focus. |
