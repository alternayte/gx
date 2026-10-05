# Badge

A small label for a status or a count.

## Usage

```gx
<badge.Badge>New</badge.Badge>
<badge.Badge variant={badge.Outline}>Draft</badge.Badge>
<badge.Badge variant={badge.Secondary}><icons.CircleCheck />Verified</badge.Badge>
<badge.Badge variant={badge.Link} href={docsRoute}>Docs</badge.Badge>
```

The variants are `Default`, `Secondary`, `Destructive`, `Outline`, `Ghost` and `Link`.
Set `Href` to render the badge as a link. A link badge has a hover state.

## Do

- Keep the text to one or two words.
- Use `Destructive` only for an error or a removal.

## Don't

- Do not use a badge as a button. Use a button.
- Do not use colour as the only signal. Write the status in the text.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to a link badge. |
| Enter | Follows a link badge. |
