# Hover Card

A richer preview on hover or focus.

## Usage

```gx
<hovercard.HoverCard trigger={gx.Text("@ada")}>
  <p>Ada Lovelace</p>
  <p>First programmer.</p>
</hovercard.HoverCard>
```

The card is CSS only. It shows on hover and on keyboard focus.

## Do

- Keep the preview short.
- Use it for a profile or a link preview.

## Don't

- Do not put a form in a hover card.
- Do not rely on a hover card for a touch-only device.

## Keyboard

| Key | Action |
| --- | --- |
| (none) | The card appears when the trigger takes keyboard focus. |
