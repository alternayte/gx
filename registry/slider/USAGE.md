# Slider

A native range control.

## Usage

```gx
<slider.Slider name="volume" value={p.Volume} label="Volume" />
```

The component is a styled native `<input type="range">`.

## Do

- Set `Min`, `Max` and `Step` for the range.
- Give the slider a `Label`.

## Don't

- Do not use a slider for an exact number. Use an input.
- Do not omit the label.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the slider. |
| Arrow keys | Changes the value by one step. |
| Home, End | Moves to the minimum or maximum. |
