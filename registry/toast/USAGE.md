# Toast

Server-pushed toasts: the toaster region and the toast.

## Usage

Render the toaster once, in the root layout.

```gx
<toast.Toaster />
```

Pass `toast.Render` to the app. The app then renders every pushed toast with the `Toast` component of this item.

```go
app := gx.New(gx.Config{Adapter: datastar.Adapter(), Toast: toast.Render})
```

An action or a form pushes a toast.

```go
return c.Toast("Saved")
```

A kind is an option. The kinds are `gx.ToastDefault`, `gx.ToastSuccess`, `gx.ToastInfo`, `gx.ToastWarning`, `gx.ToastError` and `gx.ToastLoading`.

```go
return c.Toast("Changes saved", gx.ToastSuccess)
```

Add a description, a duration or a link to a route.

```go
return c.Toast("Item added to the cart",
	gx.ToastDescription("Open the cart to check out."),
	gx.ToastDuration(8*time.Second),
	gx.ToastLink("View", route.Cart{}))
```

A toast leaves after 4 seconds. A loading toast and a `gx.ToastSticky` toast stay until the user closes them.

Give two toasts the same ID. The second toast replaces the first in place.

```go
// The first action.
return c.Toast("Uploading the file", gx.ToastLoading, gx.ToastID("upload"))

// A later action.
return c.Toast("File uploaded", gx.ToastSuccess, gx.ToastID("upload"))
```

A failed action or form pushes its error as a `gx.ToastError` toast.

The toaster shows at most three toasts. The oldest toast leaves first.

## Do

- Render one toaster in the root layout.
- Set `Toast: toast.Render` in `gx.Config`.
- Keep the text short. Put detail in the description.
- Use one ID for the steps of one operation.

## Don't

- Do not render a toaster per page.
- Do not use a toast for a value the user must not miss. Use an alert.
- Do not put the only path to a task in a toast link. The toast leaves.
- Do not write toast classes in package `gx` code. Change `Toast.gx`.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the link and the close button of a toast. The timers stop while focus is in the toaster. |
| Enter | Follows the focused link, or closes the toast on the close button. |
| Space | Closes the toast on the close button. |
| Escape | Closes the toast that holds focus. Focus returns to the element that had it before the toaster. |
