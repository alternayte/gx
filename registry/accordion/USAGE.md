# Accordion

A stack of panels that open one at a time.

## Usage

```gx
<accordion.Accordion name="faq">
  <accordion.AccordionItem name="faq" title="Is it accessible?">
    Yes. It uses the native details element.
  </accordion.AccordionItem>
</accordion.Accordion>
```

The accordion uses `<details name>`, so the browser gives the open and close behaviour.
The panel height animates in a browser that supports `interpolate-size`. Other browsers open the panel at once.
Set `Disabled` to stop an item from opening.

## Do

- Give every item of one accordion the same `Name`.
- Give each accordion on a page its own `Name`.
- Put a question in `Title` and the answer in the body.

## Don't

- Do not use an accordion for primary navigation.
- Do not open more than one item.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the summary. |
| Enter, Space | Opens or closes the item. |
