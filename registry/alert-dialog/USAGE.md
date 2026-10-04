# Alert Dialog

A modal that asks for a decision.

## Usage

```gx
<alertdialog.AlertDialog id="delete" title="Delete this item?" description="This action cannot be undone."
  confirm={button.Button(button.ButtonProps{Variant: button.Destructive, Children: gx.Text("Delete")})}>
  <:trigger><button.Button variant={button.Outline}>Delete</button.Button></:trigger>
</alertdialog.AlertDialog>
```

## Do

- Follow the question with the two outcomes.
- Name the confirm button after the action.

## Don't

- Do not close an alert dialog on an outside click.
- Do not use an alert dialog for a message with one outcome. Use a dialog.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Opens the alert dialog from the trigger. |
| Tab | Cycles through Cancel and Confirm. |
| Escape | Does not close the alert dialog. |
