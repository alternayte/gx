# Data Table Page

A page frame for a server-driven data table.

## Usage

```gx
<datatablepage.DataTablePage title="Invoices" filter={filterInput} table={table} pagination={paging} />
```

The block is copied source. The table and the paging links come from the app, so the sort, filter and page state stay typed in Go.

## Do

- Put the filter in the header row.
- Keep the table in one card.

## Don't

- Do not page on the client.
- Do not render more rows than the page size.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves to the filter, then the table links, then the paging links. |
| Enter | Follows the focused sort or page link. |
