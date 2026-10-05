# Field

Layout and labels for one form field.

## Usage

```gx
<field.FieldGroup>
  <field.Field>
    <field.FieldLabel for="email">Email</field.FieldLabel>
    <input.Input id="email" name="email" type="email" />
    <field.FieldDescription>We never share your email.</field.FieldDescription>
  </field.Field>
  <field.FieldSeparator>Or</field.FieldSeparator>
  <field.Field orientation={field.Horizontal}>
    <field.FieldContent>
      <field.FieldTitle>Product news</field.FieldTitle>
      <field.FieldDescription>One message each month.</field.FieldDescription>
    </field.FieldContent>
    <switches.Switch name="news" label="Product news" />
  </field.Field>
</field.FieldGroup>
```

`Field` has three orientations: `Vertical`, `Horizontal` and `Responsive`. A responsive field is horizontal when its `FieldGroup` is wide.

`FieldSet` and `FieldLegend` group related fields. `FieldGroup` sets the space between fields.
`FieldError` shows its children, or the distinct messages of `Errors`. It renders nothing when it has no message.

For a generated form field with rules and errors, use the `gx.FormField` control.

## Do

- Set `Invalid` on the field when a rule fails. Set `aria-invalid` on the control too.
- Put the error in `FieldError` below the control.
- Set `Disabled` on the field when its control is disabled.

## Don't

- Do not use a field for a layout grid. Use CSS.
- Do not set `For` to an id that does not exist.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
