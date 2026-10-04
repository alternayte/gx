---
description: Build a read model from events, inline or in the background.
sidebarGroup: How-to guides
title: Add a projection
---


This guide shows you how to build a read model from events with a projection.

## Write the projection

Register one handler per event type in the constructor with `On<T>`. Deedbox skips a projection when an append holds none of its event types.

<docs.Tabs sync="access">
<docs.TabItem label="EF Core"><!-- snippet: projection-ef -->
```cs
// EF Core flavour: your DbContext, enlisted in the append's transaction. Deedbox calls SaveChanges.
public sealed class CartSummaryProjection : Projection<ShopDb>
{
    public CartSummaryProjection()
    {
        On<ItemAdded>(async (e, ctx) =>
        {
            var row = await ctx.Db.CartSummaries.FindAsync([ctx.StreamId], ctx.CancellationToken)
                      ?? ctx.Db.CartSummaries.Add(new CartSummaryRow(ctx.StreamId)).Entity;
            row.ItemCount += e.Qty;
        });

        On<CheckedOut>(async (_, ctx) =>
        {
            var row = await ctx.Db.CartSummaries.FindAsync([ctx.StreamId], ctx.CancellationToken);
            row!.CheckedOut = true;
        });

        On<StreamDeleted>(async (_, ctx) =>
            await ctx.Db.CartSummaries.Where(r => r.Id == ctx.StreamId).ExecuteDeleteAsync(ctx.CancellationToken));
    }

    // A rebuild calls ResetAsync, then replays every event.
    protected override Task ResetAsync(WriteContext<ShopDb> context) =>
        context.Db.CartSummaries.ExecuteDeleteAsync(context.CancellationToken);
}
```
<!-- endSnippet -->
</docs.TabItem>
<docs.TabItem label="Dapper or ADO.NET"><!-- snippet: projection-ado -->
```cs
// ADO.NET or Dapper flavour: write through ctx.Connection and ctx.Transaction.
public sealed class CartTotals : Projection
{
    public CartTotals()
    {
        On<ItemAdded>((e, ctx) => Execute(ctx.Connection, ctx.Transaction,
            "UPDATE cart_totals SET items = items + @qty WHERE cart_id = @cart",
            ("qty", e.Qty), ("cart", ctx.StreamId)));
    }

    protected override Task ResetAsync(WriteContext context) =>
        Execute(context.Connection, context.Transaction, "DELETE FROM cart_totals");

    private static async Task Execute(DbConnection connection, DbTransaction transaction, string sql, params (string Name, object Value)[] values)
    {
        await using var command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = sql;
        foreach (var (name, value) in values)
        {
            var parameter = command.CreateParameter();
            parameter.ParameterName = name;
            parameter.Value = value;
            command.Parameters.Add(parameter);
        }

        await command.ExecuteNonQueryAsync();
    }
}
```
<!-- endSnippet -->
</docs.TabItem>
</docs.Tabs>

Override `ResetAsync` to delete what the projection wrote. A [rebuild](/how-to/rebuild-a-projection/) needs it.

## Register it with a name and a run mode

<!-- snippet: register-full -->
```cs
builder.Services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Stream<Cart>(s => s.Events<ItemAdded, CheckedOut>())
    .Projection<CartSummaryProjection>("cart_summary", Run.Inline)
    .Projection<CartTotals>("cart_totals", Run.Async)
    .Subscription<SendReceipt>("receipt_email"));
```
<!-- endSnippet -->


- The name, such as `cart_summary`, keys the projection's checkpoint. Renaming the class keeps its progress. Renaming the projection starts it again from the first event.
- A new inline projection on a store that already holds events starts as `rebuilding`. The runner applies the earlier events, then the projection runs inline. Until then, its read model is partial.
- `Run.Inline` applies the projection in the append's transaction. The read model is never behind, and a failing handler fails the append.
- `Run.Async` applies the projection in the background runner. The checkpoint commits in the same transaction as the projection's writes, so each event applies exactly once.

A projection has one run mode. Registering one class twice fails at start-up, because two registrations apply each event twice. To choose the mode, see [inline or async projections](/concepts/inline-or-async/).

## Handle large async workloads in batches

A batch projection receives each batch of its events in one call. It runs async only.

<!-- snippet: batch-projection -->
```cs
// A batch projection receives each batch of its events in one call, for bulk writes. It runs async only.
public sealed class CartArchive : BatchProjection
{
    public CartArchive() => Handles<CheckedOut>();

    protected override Task ApplyAsync(IReadOnlyList<EventEnvelope> events, WriteContext context)
    {
        // One bulk insert for the whole batch, through context.Connection and context.Transaction.
        return Task.CompletedTask;
    }
}
```
<!-- endSnippet -->


## Know what a handler sees

| Property | Inline | Async |
| --- | --- | --- |
| `EventId`, `StreamId`, `Version`, `Metadata` | yes | yes |
| `GlobalPosition` | null | yes |
| `Services` | the append's scope | a scope per batch |

Handle the built-in events `StreamDeleted` and `SubjectErased` to remove deleted and erased data from your read model.
