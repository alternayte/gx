# Separator

A visual divider between content.

## Usage

```gx
<separator.Separator />
<separator.Separator orientation={separator.Vertical} class="h-6" />
```

## Do

- Use a separator between groups that need a visual break.
- Keep the decorative default for a pure layout line.

## Don't

- Do not use a separator where whitespace is enough.
- Do not set `Decorative` to false unless the line is a real landmark.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
