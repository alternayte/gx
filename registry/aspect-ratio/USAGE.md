# Aspect Ratio

A box that keeps a fixed width to height ratio.

## Usage

```gx
<aspectratio.AspectRatio ratio="16 / 9">
  <img src={gx.URL("/hero.jpg")} alt="Hero" class="size-full object-cover" />
</aspectratio.AspectRatio>
```

## Do

- Set `Ratio` as a CSS ratio, for example `16 / 9`.
- Let the child fill the box with `size-full`.

## Don't

- Do not use an aspect ratio for text content.
- Do not nest aspect ratio boxes.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
