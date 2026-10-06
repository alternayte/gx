# Calendar

A month grid for the choice of one day.

## Usage

```gx
<calendar.Calendar id="day" name="day" value={p.Day} />
<calendar.Calendar id="start" name="start" min="2026-10-10" max="2026-10-20" weekStart={calendar.Monday} />
```

The component is a native `<input type="date">` and an island that shows a month as a grid.
The island writes the chosen day into the input, so a form sends it as `YYYY-MM-DD`.
With no script the page shows the date input of the browser.
The month and day names follow `Locale`, or the language of the page.

## Do

- Give the calendar a `Label`, or a `<label>` for its `Id`.
- Set `Min` and `Max` for a range of valid days. The server checks the value too.
- Pass `Value` from the loader for a day that depends on the time.

## Don't

- Do not use a calendar for a date of birth. Use three inputs or one text input.
- Do not use two calendars with one `Id`.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the previous-month button, the next-month button, then the grid. |
| Arrow Left, Arrow Right | Moves one day back or forward. |
| Arrow Up, Arrow Down | Moves one week back or forward. |
| Home, End | Moves to the first or the last day of the week. |
| Page Up, Page Down | Moves one month back or forward. |
| Shift + Page Up, Shift + Page Down | Moves one year back or forward. |
| Enter, Space | Chooses the day that has the focus. |
