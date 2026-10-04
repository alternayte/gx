---
description: Append events in your own connection and transaction.
sidebarGroup: How-to guides
title: Use Dapper or plain ADO.NET
---


This guide shows you how to append events in a transaction you own, with Dapper or plain ADO.NET.

## Pass your transaction

<!-- snippet: transaction-dapper -->
```cs
await using var connection = await dataSource.OpenConnectionAsync();
await using var transaction = await connection.BeginTransactionAsync();

// Your own writes, with Dapper or plain ADO.NET, on the same connection and transaction...
await store.UseTransaction(transaction).Append("cart-42", ExpectedVersion.Any, [new ItemAdded("apple", 1)]);

await transaction.CommitAsync(); // Deedbox never commits your transaction.
```
<!-- endSnippet -->


`UseTransaction` returns a store that runs every operation in your transaction. Deedbox never commits or rolls it back. Inline projections and appending hooks run in it too.

The position counter stays locked from your append until your commit. Commit soon after the append; other appends wait meanwhile.

## Write a projection with the same connection

<!-- snippet: projection-ado -->
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


## Append to one stream per transaction

A transaction that appends to two streams holds the counter from its first append while it waits for the second stream. A concurrent append can hold that stream and wait for the counter. The database then aborts one transaction as a deadlock. Nothing is lost or reordered, but that transaction fails. Append to one stream per transaction, or retry the transaction when it fails.
