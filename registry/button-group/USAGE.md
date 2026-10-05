# Button Group

A row or a column of related buttons.

## Usage

```gx
<buttongroup.ButtonGroup>
  <button.Button variant={button.Outline}>One</button.Button>
  <button.Button variant={button.Outline}>Two</button.Button>
</buttongroup.ButtonGroup>

<buttongroup.ButtonGroup>
  <button.Button variant={button.Secondary}>Copy</button.Button>
  <buttongroup.ButtonGroupSeparator />
  <button.Button variant={button.Secondary} size={button.Icon}><icons.ChevronDown /></button.Button>
</buttongroup.ButtonGroup>
```

Set `orientation={buttongroup.Vertical}` for a column.
`ButtonGroupText` shows text or a label next to a button or an input. Set `For` to make it the label of an input.
`ButtonGroupSeparator` draws a line between two buttons that have no border.
A group in a group makes a gap between the inner groups.

## Do

- Give the group an `aria-label` when its purpose is not clear from the buttons.
- Use the same variant for every button of a group.

## Don't

- Do not mix button sizes in one group.
- Do not put a separator between outline buttons. Their borders divide them.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus from one button to the next. |
| Enter | Activates the focused button. |
| Space | Activates the focused button. |
