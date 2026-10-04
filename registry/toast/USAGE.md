# Toast

A region for server-pushed toasts.

## Usage

Render the toaster once, in the root layout.

```gx
<toast.Toaster />
```

An action pushes a toast through the adapter:

```go
return c.Toast("Saved")
```

## Do

- Render one toaster in the root layout.
- Keep the message short.

## Don't

- Do not render a toaster per page.
- Do not use a toast for a value the user must not miss. Use an alert.

## Keyboard

| Key | Action |
| --- | --- |
| None | The toaster is an aria-live region and takes no focus. |
