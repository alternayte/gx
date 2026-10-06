# Command

A list of commands with a search input.

## Usage

```gx
<command.Command id="palette" groups={p.Groups} />
<command.Command id="pages" label="Go to" placeholder="Search pages..." groups={p.Pages} />
```

The server renders the whole list. An item with an `Href` is a link, and an item with no `Href` is a button.
The island adds the search input. It filters the items by their text and their `Keywords`.
A button item sends the `command-select` event with its `Value` when the user runs it.
With no script the page shows the list with no search input, and each link works.

## Do

- Give each group a `Heading`.
- Add `Keywords` for the words that a user types for an item.
- Put the command in a dialog for a command palette.

## Don't

- Do not use it for the choice of a form value. Use a combobox.
- Do not put more than about fifty items in one list.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the search input. |
| Type | Filters the items. |
| Arrow Down, Arrow Up | Moves to the next or previous item. |
| Home, End | Moves to the first or last item. |
| Enter | Runs the active item: follows the link or sends the event. |
