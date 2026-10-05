# Select

A native list of exclusive choices.

## Usage

```gx
<selectbox.Select name="fruit" label="Fruit" placeholder="Select a fruit" class="w-48">
  <selectbox.SelectGroup label="Fruits">
    <selectbox.SelectOption value="apple">Apple</selectbox.SelectOption>
    <selectbox.SelectOption value="banana">Banana</selectbox.SelectOption>
  </selectbox.SelectGroup>
  <selectbox.SelectSeparator />
  <selectbox.SelectOption value="carrot" selected={true}>Carrot</selectbox.SelectOption>
</selectbox.Select>
```

The component is a styled native `<select>`, so the keyboard, the option list and the mobile picker come from the browser. `Size` is `selectbox.Md` (default) or `selectbox.Sm`. `Class` sets the width of the control; without it the control is as wide as its longest option. `Attrs` go on the `<select>`.

`Placeholder` shows muted text until the user selects an option. `SelectGroup` is an `<optgroup>` and `SelectSeparator` is an `<hr>` between options.

## Do

- Mark one option with `Selected`, or set a `Placeholder`.
- Pair the select with a label, or set `Label`.
- Set `aria-invalid` through `Attrs` when the value fails validation.

## Don't

- Do not use a select for fewer than four options. Use a radio group.
- Do not leave the select without a name in a form.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the select. |
| Arrow keys | Moves through the options. |
| A letter | Selects the next option that starts with the typed text. |
| Enter, Space | Opens or commits the choice. |
