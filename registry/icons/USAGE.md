# Icons

The Lucide icons that the registry components use.

Each icon is an inline SVG that takes the text colour. The paths come from Lucide (ISC licence, https://lucide.dev/license).

## Usage

```gx
<icons.Check class="size-4" />
<icons.X class="size-4" label="Close" />
```

## Do

- Set the size with a `size-*` class.
- Set `label` when the icon is the only content of a control.

## Don't

- Do not add an icon to this package for one app. Run `gx icons pin lucide@<version>` for the full set.
- Do not set `label` on a decorative icon. A decorative icon stays hidden from assistive technology.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
