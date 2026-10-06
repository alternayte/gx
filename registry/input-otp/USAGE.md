# Input OTP

A one-time code input with one slot for each character.

## Usage

```gx
<label for="code">Verification code</label>
<inputotp.InputOTP id="code" name="code" />
<inputotp.InputOTP id="pin" name="pin" length={4} label="PIN" />
<inputotp.InputOTP id="code" name="code" group={3} label="Verification code" />
```

The component is one native `<input>` and an island that shows its value as slots.
A form sends the value of the input. A paste and the code from a text message fill the slots.
With no script the page shows the plain input.

## Do

- Give the input a `<label>` or a `Label`.
- Set `Length` to the length of the code that you send.
- Use `Group` for a long code, for example 3 for a code of 6 digits.

## Don't

- Do not use it for a password. Use an input of type `password`.
- Do not use two components with one `Id`.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the input. |
| 0 to 9 | Types a digit into the active slot. |
| Backspace | Removes the digit before the caret. |
| Arrow Left, Arrow Right | Moves the caret. |
| Paste | Fills the slots with the digits of the text. |
