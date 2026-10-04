# Avatar

A user image with a fallback.

## Usage

```gx
<avatar.Avatar src={gx.URL(user.Image)} alt={user.Name} fallback="NA" />
```

## Do

- Always set `Alt` when the image carries meaning.
- Set `Fallback` to the initials of the user.

## Don't

- Do not build an image URL from a raw string. Use `gx.URL`.
- Do not use an avatar as the only label of an action.

## Keyboard

This component is static. It takes no focus and has no key bindings.
