# Data Table

A server-driven table with typed columns, sort headers and paging.

## Usage

```go
type Row struct {
  Name  string
  Total int
}

var columns = []datatable.Column[Row]{
  {Key: "name", Label: "Name", Sort: "name", Cell: func(r Row) gx.Node { return gx.Text(r.Name) }},
  {Key: "total", Label: "Total", Cell: func(r Row) gx.Node { return gx.Text(strconv.Itoa(r.Total)) }},
}

// In the view:
datatable.DataTable(datatable.DataTableProps[Row]{
  Columns: columns,
  Rows:    p.Rows,
  Page:    datatable.Page{Number: p.Page, Size: 25, Total: 10000},
  Sort:    p.Sort,
  Dir:     p.Dir,
  Href: func(sort, dir string, page int) gx.URL {
    return invoices.List{Sort: sort, Dir: dir, Page: page}.URL()
  },
})
```

## Do

- Sort, filter and page on the server through a typed GET route.
- Pass the current `Sort` and `Dir` back into the props.

## Don't

- Do not page on the client.
- Do not render more rows than the page size.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves between the sort headers and the page links. |
| Enter | Sorts by the focused header or follows a page link. |
