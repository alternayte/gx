# Scroll Area

A box with a styled scrollbar.

## Usage

```gx
<scrollarea.ScrollArea class="h-64 rounded-md border">
  <div class="p-4">Long content.</div>
</scrollarea.ScrollArea>
```

The area is a native scroller with a thin scrollbar in the border colour.
It scrolls on both axes. The browser draws the scrollbar.

## Do

- Set a height on the area.
- Put the padding on the content, not on the area.
- Keep the scroll on one axis.

## Don't

- Do not nest two scroll areas.
- Do not use a scroll area for the whole page.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the area. |
| Arrow keys, Page keys | Scroll the area when it holds focus. |
