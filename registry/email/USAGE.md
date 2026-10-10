# Email

The parts of an HTML email. Each part has inline styles and table layout, which each email client reads. No part has a class, a signal or a script.

## Usage

Write the email as a component in a package with the name `email`. Then render it with `gx.RenderEmail`.

```gx
<email.Document preview="Your order is on its way.">
  <email.Section>
    <email.Heading>Your order is on its way</email.Heading>
    <email.Text>Thank you for your order.</email.Text>
    <email.Button href={gx.URL(route.Order{ID: p.ID}.URL())}>See the order</email.Button>
  </email.Section>
  <email.Divider />
  <email.Section card>
    <email.Row>
      <email.Column>Tea, 2 boxes</email.Column>
      <email.Column width={120} align={email.Right}>12.00</email.Column>
    </email.Row>
  </email.Section>
  <email.Image src={gx.URL("/logo.png")} alt="The logo of the shop" width={120} height={40} />
  <email.Text muted>You get this email because you have an account.</email.Text>
</email.Document>
```

```go
msg, err := gx.RenderEmail(OrderEmail(props), gx.EmailOptions{
	BaseURL: "https://shop.example",
	Tokens:  gxstyles.LightTokens(),
})
```

`msg.HTML` is the HTML document and `msg.Text` is the same content as plain text. Give the two parts to your mail library. Gx sends no email.

## Parts

| Part | Use |
| --- | --- |
| `Document` | The background and the centre column. It is the root of each email. `preview` is the text that an inbox shows after the subject. |
| `Section` | A group of parts. `card` gives it a border and padding. |
| `Row` | One row of columns. |
| `Column` | One cell of a row. `width` is in pixels; zero shares the row. |
| `Text` | A paragraph. `muted` is for a note. |
| `Heading` | A heading of `level` 1, 2 or 3. |
| `Button` | A link with the look of a button. |
| `Image` | An image with its size and its `alt` text. |
| `Divider` | A line between two sections. |

The styles read the theme tokens with `var()`: `--background`, `--foreground`, `--card`, `--primary`, `--primary-foreground`, `--muted-foreground`, `--border` and `--radius`. `gx.RenderEmail` gives each one the value from `gxstyles.LightTokens()` or `gxstyles.DarkTokens()`, so the email has no CSS variable.

## Do

- Put each part inside one `Document`.
- Put only `Column` parts in a `Row`.
- Give each `Image` its `alt` text, its width and its height. Many email clients block images at first.
- Use a site path or a typed link for an address. `gx.RenderEmail` makes it an absolute URL.

## Don't

- Do not add a `class` attribute or a signal to a part. The compiler reports GX6010.
- Do not use a component of a page, for example `button.Button`, in an email. Its classes have no effect there, and `gx.RenderEmail` returns an error.
- Do not put text directly in a `Row`. A table row takes only cells.

## Keyboard

| Key | Action |
| --- | --- |
| None | The parts are static. A link and a button get the focus as each link does. |
