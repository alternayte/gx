# starlight

The Starlight theme for a docs site. It gives a Gx site the look of the
default Starlight theme: the colours in light and dark, the fonts, the
layout of the page, the prose, the code frames and the components of the
`docs` kit.

The item is one stylesheet, `app/starlight.css`. The `starlight-shell`
block writes the page structure that the stylesheet styles.

The values come from `@astrojs/starlight` (MIT licence, Astro contributors).

## Use

Import the stylesheet in `app/theme.css`, after the Tailwind import:

```css
@import "tailwindcss";
@import "./starlight.css";

@source "../.gx/classes.txt";

@custom-variant dark (&:where(.dark, .dark *));

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-border: var(--border);
}
```

The stylesheet sets the shadcn tokens (`--background`, `--primary`,
`--border` and the others) from the Starlight colours, so each registry
component gets colours that fit. Do not set them again in `theme.css`.

Change the accent colour or a font with the Starlight variables:

```css
:root {
  --sl-font: "Inter";
  --sl-color-accent: hsl(150, 80%, 35%);
}
```

Add the code stylesheet of Gx in `main.go`. The theme sets its colours:

```go
gx.SetStylesheet(append(gxstyles.CSS(), content.CodeCSS()...))
```

## Do

- Use the `starlight-shell` block for the pages. The layout rules need its markup.
- Set a colour for dark mode in a `.dark` rule, and again in `@media (prefers-color-scheme: dark)` for `:root:not(.light)`.

## Don't

- Do not use this theme with the `docs-shell` block. That block has its own layout.
- Do not edit the token values in `starlight.css`. Set them in `theme.css`, so `gx update starlight` merges cleanly.

## Keyboard

The theme adds no behaviour and no key.

| Key | Action |
| --- | --- |
| Tab | Shows the focus ring of the browser on each link and control. |
