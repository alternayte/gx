# Hover Card

A richer preview on hover or focus.

## Usage

```gx
<hovercard.HoverCard trigger={gx.Text("@ada")} class="text-sm">
  <p class="font-medium">Ada Lovelace</p>
  <p class="text-muted-foreground">First programmer.</p>
</hovercard.HoverCard>
```

The card is CSS only. It shows on hover and on keyboard focus. It opens after 700 ms and closes after 300 ms, so the pointer can move from the trigger into the card.

`Align` lines the card up with the trigger: `hovercard.Start` (default), `hovercard.Center` or `hovercard.End`. The card does not move to stay inside the page, so select the alignment that fits the place of the trigger.

The card fades, zooms and slides in. A user who asks for reduced motion gets no transition.

## Do

- Keep the preview short.
- Use it for a profile or a link preview.

## Don't

- Do not put a form in a hover card.
- Do not rely on a hover card for a touch-only device.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | The card appears when the trigger takes keyboard focus. |
