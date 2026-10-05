# Popover

A small panel anchored to a trigger.

## Usage

```gx
<popover.PopoverTrigger id="dimensions">Open</popover.PopoverTrigger>
<popover.Popover id="dimensions">
  <popover.PopoverHeader>
    <popover.PopoverTitle>Dimensions</popover.PopoverTitle>
    <popover.PopoverDescription>Set the dimensions for the layer.</popover.PopoverDescription>
  </popover.PopoverHeader>
</popover.Popover>
```

The element uses the native Popover API. The trigger is a `button.Button`; `Variant` and `Size` select its style, and the default is `button.Outline`.

`Align` lines the popover up with the trigger: `popover.Center` (default), `popover.Start` or `popover.End`. The popover always opens below the trigger. A browser without anchor positioning shows the popover at its place in the document flow.

The popover fades and zooms in and out. Safari shows the enter transition only. A user who asks for reduced motion gets no transition.

## Do

- Give the popover and the trigger the same `Id`.
- Put the popover directly after its trigger in the markup.
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
