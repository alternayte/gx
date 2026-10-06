# Date Picker

A button that opens a calendar in a popover.

## Usage

```gx
<label for="due">Due date</label>
<datepicker.DatePicker id="due" name="due" value={p.Due} />
<datepicker.DatePicker id="start" name="start" min="2026-10-10" max="2026-10-20" placeholder="Start date" />
```

The component is a popover with a calendar. The calendar holds a native `<input type="date">`, so a form sends the day as `YYYY-MM-DD`.
The button shows the chosen day in the language of the user. A choice closes the popover.
With no script the button opens the popover, and the popover shows the date input of the browser.

## Do

- Give the date picker a `<label>` for its `Id`.
- Set `Min` and `Max` for a range of valid days. The server checks the value too.
- Use a `Placeholder` that names the date, for example "Start date".

## Don't

- Do not use a date picker for a date of birth. Use three inputs or one text input.
- Do not put a date picker in a popover.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the button. |
| Enter, Space | Opens the calendar from the button, or chooses the day that has the focus. |
| Arrow keys | Moves one day or one week in the calendar. |
| Page Up, Page Down | Moves one month back or forward. |
| Escape | Closes the calendar and moves focus to the button. |
