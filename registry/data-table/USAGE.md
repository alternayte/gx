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
    return gx.URL(route.List{Sort: sort, Dir: dir, Page: page}.URL())
  },
})
```

## Live filter

A search field filters the table while the user types. A GET action reads the signals and patches the table. The page route still holds the filter in its query, so a page load and each link keep it.

The route of the page and the route of the action:

```go
type List struct {
  gx.Route `GET /invoices`
  Q        string `query:"q"`
  Sort     string `query:"sort"`
  Dir      string `query:"dir"`
  Page     int    `query:"page" default:"1"`
}

type Search struct {
  gx.Route `GET /invoices/search`
  Q        string `signal:"q"`
  Sort     string `signal:"sort"`
  Dir      string `signal:"dir"`
}

// The browser controls each signal value.
func (in *Search) Rules() gx.Rules {
  return gx.Rules{
    gx.Field(&in.Q, gx.MaxLen(100)),
    gx.Field(&in.Sort, gx.OneOf("", "name", "total")),
    gx.Field(&in.Dir, gx.OneOf("", "asc", "desc")),
  }
}
```

The view. The first value of each signal comes from a prop, so a page load starts the signals from the query. The table is in a fragment, so the action can patch it:

```gx
props {
  Table gx.Node
  Q     string = ""
  Sort  string = ""
  Dir   string = ""
}

signals {
  Q    string = p.Q
  Sort string = p.Sort
  Dir  string = p.Dir
}

<input type="search" aria-label="Filter" class={input.Class} bind:value={$Q} on:input.debounce(300ms)={route.Search{}} />
table := p.Table
<div #results(table gx.Node)>
  {table}
</div>
```

The handlers. One function builds the table for the page and for the action. Its `Href` puts `q` in each link, so a sort or page link keeps the filter:

```go
func table(q, sort, dir string, page int) gx.Node {
  rows := filterAndSort(q, sort, dir)
  return datatable.DataTable(datatable.DataTableProps[Row]{
    Columns: columns,
    Rows:    rows,
    Page:    datatable.Page{Number: page, Size: 25, Total: len(rows)},
    Sort:    sort,
    Dir:     dir,
    Href: func(sort, dir string, page int) gx.URL {
      return gx.URL(route.List{Q: q, Sort: sort, Dir: dir, Page: page}.URL())
    },
  })
}

var ListPage = gx.Page(func(c *gx.Ctx, in route.List) (ListViewProps, error) {
  return ListViewProps{Table: table(in.Q, in.Sort, in.Dir, in.Page), Q: in.Q, Sort: in.Sort, Dir: in.Dir}, nil
}, ListView)

var Search = gx.Action(func(c *gx.Ctx, in route.Search) error {
  key := gx.ScopeKey(gx.Scope(c.R), "invoices.ListView")
  return c.Patch(ListViewResults(key, table(in.Q, in.Sort, in.Dir, 1)))
})
```

- Use a plain `<input>` with `input.Class`. A client directive on a component element, such as `<input.Input bind:value={$Q} />`, is the error GX2003.
- The fragment of a component with signals takes the instance key as its first parameter. `gx.ScopeKey` reads it from the request.
- The action patches the table only. The URL in the address bar keeps the old query until the next navigation.

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
