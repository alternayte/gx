# Popover

A small panel anchored to a trigger.

## Usage

```gx
<popover.PopoverTrigger id="menu">Open</popover.PopoverTrigger>
<popover.Popover id="menu">
  <p>Place content for the popover here.</p>
</popover.Popover>
```

The element uses the native Popover API. A browser without anchor positioning centers the panel instead of anchoring it.

## Do

- Give the popover and the trigger the same `Id`.
- Keep the content short.

## Don't

- Do not put a form with many fields in a popover.
- Do not open a popover from another popover.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Toggles the popover. |
| Tab | Moves focus into the popover. |
| Escape | Closes the popover. |
