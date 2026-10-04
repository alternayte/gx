# Button Group

A row of related buttons.

## Usage

```gx
<buttongroup.ButtonGroup>
  <button.Button variant={button.Outline}>One</button.Button>
  <button.Button variant={button.Outline}>Two</button.Button>
</buttongroup.ButtonGroup>
```

## Do

- Group buttons that act on the same object.
- Give each button a distinct label.

## Don't

- Do not group unrelated actions.
- Do not nest a button group inside another button group.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus between the buttons. |
| Enter | Activates the focused button. |
