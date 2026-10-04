# Field

Layout and labels for one form field.

## Usage

```gx
<field.Field>
  <field.FieldLabel for="email">Email</field.FieldLabel>
  <input.Input name="email" type="email" />
  <field.FieldDescription>We never share your email.</field.FieldDescription>
</field.Field>
```

For a generated form field with rules and errors, use the `gx.FormField` control.

## Do

- Set `Invalid` on the field when a rule fails.
- Put the error in `FieldError` below the control.

## Don't

- Do not use a field for a layout grid. Use CSS.
- Do not set `For` to an id that does not exist.

## Keyboard

This component is static. It adds no key bindings.
