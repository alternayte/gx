# Card

A bordered surface for grouped content.

## Usage

```gx
<card.Card title="Team" description="Manage the members." footer={saveButton}>
  <:action><button.Button variant={button.Link}>Invite</button.Button></:action>
  <p>Card body.</p>
</card.Card>
```

`Title` and `Description` make the header. `Action` sits at the top right of the header.
The children are the content. `Footer` is the last row.

## Do

- Use one card for one subject.
- Put the main action of the card in `Footer`.

## Don't

- Do not nest a card in a card.
- Do not use a card for a page layout. Use CSS.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
