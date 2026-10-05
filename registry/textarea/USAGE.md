# Textarea

A multi-line text control.

## Usage

```gx
<textarea.Textarea name="note" placeholder="Tell us more." />
```

The textarea grows with its content. It starts at two lines.
`Rows` sets the height only in a browser without `field-sizing`.
Set `aria-invalid="true"` through `Attrs` to show the error border.

## Do

- Use a textarea for a message longer than one line.
- Set a `max-h-` class when a long text must scroll.

## Don't

- Do not use a textarea for a single value.
- Do not disable the resize handle unless the layout needs it.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the textarea. |
| Enter | Inserts a new line. |
