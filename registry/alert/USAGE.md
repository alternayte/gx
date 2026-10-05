# Alert

A callout for an important message.

## Usage

```gx
<alert.Alert title="Heads up">
  You can add components to your app.
</alert.Alert>

<alert.Alert variant={alert.Destructive} title="Error" icon={<icons.TriangleAlert />}>
  Your session expired. Sign in again.
</alert.Alert>
```

`Icon` takes one svg icon. The alert opens a column for the icon when it has one.
`Title` shows one line. The children are the description.

## Do

- Put the main fact in `Title`.
- Use `Destructive` only for an error.

## Don't

- Do not use an alert for a transient message. Use a toast.
- Do not wrap the icon in another element. The alert finds the svg as its direct child.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
