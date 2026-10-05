# Spinner

An indicator for work in progress.

## Usage

```gx
<spinner.Spinner />
<spinner.Spinner class="size-6 text-muted-foreground" label="Saving" />
<button.Button attrs={gx.Attrs{gx.Bool("disabled", true)}}><spinner.Spinner />Saving</button.Button>
```

The spinner is the loader icon with a rotation. Set the size and the colour with `Class`.
`Label` is the name a screen reader announces. The default is `Loading`.

## Do

- Show the spinner next to the text of the action that is in progress.
- Change `Label` when the work is not a load.

## Don't

- Do not show a spinner for a wait that has a known length. Use a progress bar.
- Do not block the whole page with a spinner.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
