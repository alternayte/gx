---
description: Expose a projection's tables through Hot Chocolate, with DataLoaders so a list costs a fixed number of queries.
sidebarGroup: How-to guides
title: Serve a read model over GraphQL
---


This guide shows you how to serve a read model over GraphQL with [Hot Chocolate](https://chillicream.com/docs/hotchocolate). It uses the read model of the [manuscript tutorial](/tutorials/manuscript-versions/), but the pattern fits any projection. GraphQL only reads here; commands still go through `Execute`.

```sh
dotnet add package HotChocolate.AspNetCore
```

## Give GraphQL its own database contexts

GraphQL runs resolvers in parallel, and one `DbContext` cannot serve two queries at once. Register a factory with `AddDbContextFactory`, and open a context for each query, as the tutorial's `ManuscriptQueries` does. `AddDbContextFactory` also registers the context itself, so the projection works unchanged.

## Write the root fields

The root fields call the same query class as the REST API, so both return the same shapes.

<!-- snippet: graphql-query -->
```cs
// The root fields. The list returns summaries; the fields below load the rest only when a client asks for it.
public sealed class ManuscriptQuery
{
    public Task<ManuscriptPage> Manuscripts(ManuscriptQueries q, string? search, Status? status, string? after, int limit = 20, CancellationToken ct = default) =>
        q.List(search, status, after, Math.Clamp(limit, 1, 100), ct);

    public Task<ManuscriptView?> Manuscript(string id, ManuscriptQueries q, CancellationToken ct) => q.Manuscript(id, ct);

    public Task<VersionView?> Version(string id, int number, ManuscriptQueries q, CancellationToken ct) => q.Version(id, number, ct);

    public Task<List<SectionChange>> Changes(string id, int from, int to, ManuscriptQueries q, CancellationToken ct) => q.Changes(id, from, to, ct);
}
```
<!-- endSnippet -->


## Load related data with DataLoaders

Without DataLoaders, a list of 20 manuscripts with their authors and versions costs 41 queries: one for the page, then two for each manuscript. That is the N+1 problem. A DataLoader collects the keys of all the resolvers in one request, then reads them in one query.

<!-- snippet: graphql-dataloaders -->
```cs
// Each loader collects the keys of one request's resolvers, then reads them all in one query.
public sealed class DocumentByIdDataLoader(IDbContextFactory<PublishingDb> contexts, IBatchScheduler scheduler, DataLoaderOptions options)
    : BatchDataLoader<string, ManuscriptView>(scheduler, options)
{
    protected override async Task<IReadOnlyDictionary<string, ManuscriptView>> LoadBatchAsync(IReadOnlyList<string> ids, CancellationToken ct)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        var rows = await db.Manuscripts.AsNoTracking().Where(m => ids.Contains(m.Id)).Select(m => new { m.Id, m.Document }).ToListAsync(ct);
        return rows.ToDictionary(r => r.Id, r => Documents.Read(r.Document));
    }
}

public sealed class VersionsByManuscriptIdDataLoader(IDbContextFactory<PublishingDb> contexts, IBatchScheduler scheduler, DataLoaderOptions options)
    : GroupedDataLoader<string, VersionSummary>(scheduler, options)
{
    protected override async Task<ILookup<string, VersionSummary>> LoadGroupedBatchAsync(IReadOnlyList<string> ids, CancellationToken ct)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        var versions = await db.Versions.AsNoTracking().Where(v => ids.Contains(v.ManuscriptId)).OrderBy(v => v.Number)
            .Select(v => new VersionSummary(v.ManuscriptId, v.Number, v.Stage, v.BasedOn, v.Reason, v.FrozenAt)).ToListAsync(ct);
        return versions.ToLookup(v => v.ManuscriptId);
    }
}
```
<!-- endSnippet -->


The fields on each manuscript in a list ask the DataLoaders, so a field costs a query only when a client asks for it:

<!-- snippet: graphql-fields -->
```cs
// Fields on each manuscript in a list. Every field asks a DataLoader, so a page of 20 costs one query per loader, not 20.
[ExtendObjectType(typeof(ManuscriptSummary))]
public sealed class ManuscriptSummaryFields
{
    public async Task<IReadOnlyList<AuthorView>> GetAuthors([Parent] ManuscriptSummary m, DocumentByIdDataLoader documents, CancellationToken ct) =>
        (await documents.LoadRequiredAsync(m.Id, ct)).Authors;

    public async Task<VersionView?> GetLatest([Parent] ManuscriptSummary m, DocumentByIdDataLoader documents, CancellationToken ct) =>
        (await documents.LoadRequiredAsync(m.Id, ct)).Latest;

    public async Task<VersionView?> GetPublished([Parent] ManuscriptSummary m, DocumentByIdDataLoader documents, CancellationToken ct) =>
        (await documents.LoadRequiredAsync(m.Id, ct)).Published;

    public async Task<IReadOnlyList<RoundView>> GetRounds([Parent] ManuscriptSummary m, DocumentByIdDataLoader documents, CancellationToken ct) =>
        (await documents.LoadRequiredAsync(m.Id, ct)).Rounds;

    public async Task<VersionSummary[]> GetVersions([Parent] ManuscriptSummary m, VersionsByManuscriptIdDataLoader versions, CancellationToken ct) =>
        await versions.LoadRequiredAsync(m.Id, ct);
}
```
<!-- endSnippet -->


| Query | Database queries |
| --- | --- |
| `manuscripts { items { id title } }` | 1 |
| `manuscripts { items { id authors { name } latest { number } } }` | 2: the page, then all the documents |
| `manuscripts { items { id authors { name } versions { number } } }` | 3: the page, the documents, then all the versions |

The number stays the same for a page of 5 or a page of 100.

## Register the schema and map the endpoint

<!-- snippet: graphql-register -->
```cs
builder.Services.AddGraphQLServer()
    .AddQueryType<ManuscriptQuery>()
    .AddTypeExtension<ManuscriptSummaryFields>()
    .AddDataLoader<DocumentByIdDataLoader>()
    .AddDataLoader<VersionsByManuscriptIdDataLoader>();
```
<!-- endSnippet -->


<!-- snippet: graphql-map -->
```cs
app.MapGraphQL();   // POST /graphql
```
<!-- endSnippet -->


Ask for a page of manuscripts and their related data:

```graphql
{
  manuscripts(search: "frozen", limit: 20) {
    items { id title status authors { name } latest { number stage } versions { number stage } }
    next
  }
}
```

Hot Chocolate writes enum values in capitals with underscores, such as `SUBMITTED_UNDER_REVIEW`. Pass `next` as `after` to get the next page.
