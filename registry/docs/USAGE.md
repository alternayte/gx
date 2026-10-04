# docs

The Gx docs kit. Copy it into an app and render documentation pages with
typed components.

## Use

```go
var Docs = gx.Collection[DocMeta]("content/docs").Components(
    docs.Aside, docs.Tabs, docs.TabItem, docs.Steps, docs.Card, docs.CardGrid,
    docs.LinkCard, docs.LinkButton, docs.Badge, docs.FileTree, docs.Code, docs.LLMSkip,
)
```

In Markdown:

```markdown
<docs.Steps>
1. Create the page.
2. Mount the route.
</docs.Steps>

<docs.Tabs sync="db">
  <docs.TabItem label="Postgres">...</docs.TabItem>
  <docs.TabItem label="SQL Server">...</docs.TabItem>
</docs.Tabs>

<docs.Code code={gx.CodeFile("main.go", "1-20")} />

<docs.LLMSkip>This section stays out of llms.txt.</docs.LLMSkip>
```

## Do

- Name the sync key when two tab groups must move together.
- Wrap a section in `docs.LLMSkip` when agents must not read it.

## Don't

- Do not use `docs.Code` with a file outside the module.
- Do not repeat a label inside one tab group.

## Keyboard

| Key | Action |
| --- | --- |
| Enter, Space | Switches a docs.Tabs tab. |
| Ctrl/Cmd K | Opens the docs-shell search dialog when the shell is present. |
