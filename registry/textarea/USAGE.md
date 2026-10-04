# Textarea

A multi-line text control.

## Usage

```gx
<textarea.Textarea name="note" rows={6} placeholder="Tell us more." />
```

## Do

- Use a textarea for a message longer than one line.
- Set `Rows` to the expected size.

## Don't

- Do not use a textarea for a single value.
- Do not disable the resize handle unless the layout needs it.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the textarea. |
| Enter | Inserts a new line. |
