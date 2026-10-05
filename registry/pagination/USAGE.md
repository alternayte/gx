# Pagination

Links between pages of a result set.

## Usage

```gx
<pagination.Pagination>
  <pagination.PaginationContent>
    <pagination.PaginationItem>
      <pagination.PaginationPrevious href={products.List{Page: p.Page - 1}} />
    </pagination.PaginationItem>
    <pagination.PaginationItem>
      <pagination.PaginationLink href={products.List{Page: 1}} active={p.Page == 1}>1</pagination.PaginationLink>
    </pagination.PaginationItem>
    <pagination.PaginationItem>
      <pagination.PaginationNext href={products.List{Page: p.Page + 1}} />
    </pagination.PaginationItem>
  </pagination.PaginationContent>
</pagination.Pagination>
```

A link has the look of a ghost button. The current page has the look of an outline button.
`PaginationPrevious` and `PaginationNext` show a chevron. They hide their text on a narrow screen.
`PaginationLink` takes a `button.Size`. The default is `button.Icon`.

## Do

- Pass typed route values to `Href`.
- Set `Active` on the current page link.

## Don't

- Do not use a pagination link for an action.
- Do not show more links than fit on one line.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between the links. |
| Enter | Follows the link. |
