# Sheet

A panel that slides in from an edge.

## Usage

```gx
<sheet.Sheet id="filters" title="Filters" side={sheet.Right}>
  <:trigger><button.Button variant={button.Outline}>Filters</button.Button></:trigger>
  <p>Filter controls.</p>
</sheet.Sheet>
```

## Do

- Use the right side for a detail panel and the bottom for a small action sheet.
- Keep one main task in the sheet.

## Don't

- Do not open a sheet over another overlay.
- Do not use a sheet for a destructive confirmation. Use an alert dialog.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Opens the sheet from the trigger. |
| Tab | Cycles through the controls of the sheet. |
| Escape | Closes the sheet. |
