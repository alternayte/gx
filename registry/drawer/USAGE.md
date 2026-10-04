# Drawer

A bottom panel for a short task.

## Usage

```gx
<drawer.Drawer id="share" title="Share this page">
  <:trigger><button.Button variant={button.Outline}>Share</button.Button></:trigger>
  <p>Choose a destination.</p>
</drawer.Drawer>
```

## Do

- Use a drawer for a short, focused task on a small screen.
- Keep the footer actions in one column.

## Don't

- Do not put a long form in a drawer.
- Do not nest a drawer in a sheet.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Opens the drawer from the trigger. |
| Tab | Cycles through the controls of the drawer. |
| Escape | Closes the drawer. |
