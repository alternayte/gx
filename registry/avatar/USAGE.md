# Avatar

A user image with a fallback.

## Usage

```gx
<avatar.Avatar src={gx.URL(user.Image)} alt={user.Name} fallback="NA" />
<avatar.Avatar fallback="NA" size={avatar.Lg} badge={<avatar.AvatarBadge />} />

<avatar.AvatarGroup>
  <avatar.Avatar fallback="NA" />
  <avatar.Avatar fallback="AL" />
  <avatar.AvatarGroupCount>+3</avatar.AvatarGroupCount>
</avatar.AvatarGroup>
```

The sizes are `Sm`, `Md` and `Lg`. The fallback, the badge and the group count follow the size of the avatar.
`AvatarBadge` takes an icon as its child. A small avatar hides the icon.

## Do

- Always set `Alt` when the image carries meaning.
- Set `Fallback` to the initials of the user.
- Use one size for every avatar of a group.

## Don't

- Do not build an image URL from a raw string. Use `gx.URL`.
- Do not use an avatar as the only label of an action.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
