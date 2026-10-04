# Label

A caption for a form control.

## Usage

```gx
<label.Label for="email">Email</label.Label>
<input id="email" name="email" type="email" />
```

## Do

- Point `For` at the id of the control.
- Keep the label short.

## Don't

- Do not use placeholder text as the only label.
- Do not point `For` at an element that is not a form control.

## Keyboard

| Key | Action |
| --- | --- |
| (none) | Clicking the label moves focus to its control. |
