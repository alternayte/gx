# Combobox

A select with a text filter.

## Usage

```gx
<label for="framework">Framework</label>
<combobox.Combobox id="framework" name="framework" options={p.Frameworks} value={p.Framework} />
<combobox.Combobox id="city" name="city" options={p.Cities} label="City" placeholder="Select a city" />
```

The component is a native `<select>` and an island that shows it as a text input with a list.
The island writes the chosen value into the select, so a form sends it.
With no script the page shows the select of the browser.
The filter finds each option whose label holds the typed text.

## Do

- Give the combobox a `<label>` for its `Id`, or a `Label`.
- Use it for a list of more than about seven options. Use a select for a short list.
- Check the value on the server. The browser can send any text.

## Don't

- Do not use it for free text. Use an input.
- Do not use two comboboxes with one `Id`.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the input. |
| Type | Opens the list and filters the options. |
| Arrow Down, Arrow Up | Opens the list and moves to the next or previous option. |
| Home, End | Moves to the first or last option. |
| Enter | Chooses the active option and closes the list. |
| Escape | Closes the list and keeps the chosen option. |
