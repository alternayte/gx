# Dialog

A modal overlay with a trigger.

## Usage

```gx
<dialog.Dialog id="edit" title="Edit profile" footer={saveButton}>
  <:trigger><button.Button variant={button.Outline}>Open</button.Button></:trigger>
  <p>Dialog body.</p>
</dialog.Dialog>
```

## Do

- Give every dialog a unique `Id`.
- Put the main action in `Footer` and a close control in the corner.

## Don't

- Do not open a dialog from another dialog.
- Do not put a long form in a dialog. Use a page.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Opens the dialog from the trigger. |
| Tab | Cycles through the controls of the dialog. |
| Escape | Closes the dialog. |
