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

## Do

- Give every item the same `Name`.
- Put a question in `Title` and the answer in the body.

## Don't

- Do not use an accordion for primary navigation.
- Do not open more than one item.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the summary. |
| Enter, Space | Opens or closes the item. |
