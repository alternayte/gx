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
