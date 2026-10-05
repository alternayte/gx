# Kbd

A key or a key combination.

## Usage

```gx
<kbd.Kbd>Esc</kbd.Kbd>

<kbd.KbdGroup>
  <kbd.Kbd>Ctrl</kbd.Kbd>
  <span>+</span>
  <kbd.Kbd>K</kbd.Kbd>
</kbd.KbdGroup>
```

`KbdGroup` keeps the keys of one shortcut on one line.

## Do

- Write the key as it is printed on the keyboard.
- Put one key in one `Kbd`.

## Don't

- Do not use a kbd for a button. It does not take a click.
- Do not put a sentence in a kbd.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
