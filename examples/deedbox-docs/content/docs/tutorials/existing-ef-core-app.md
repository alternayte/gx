---
description: Event-source one aggregate of an EF Core app, with its read model in the same transaction.
order: 2
sidebarGroup: Tutorials
title: Add Deedbox to an existing EF Core app
---


In this tutorial, you move one aggregate of an existing EF Core app to events. Your other tables stay in EF Core. A read model table, updated by an inline projection, commits in the same transaction as the events. It takes under an hour.

The example app has a `ShopDb` DbContext. You event-source its cart and keep a `CartSummaries` table for queries.

<docs.Steps>

1. Add the packages.

   ```sh
   dotnet add package Deedbox.Postgres
   dotnet add package Deedbox.EntityFrameworkCore
   ```

   Use `Deedbox.SqlServer` for SQL Server. The rest of this tutorial is the same.

2. Write the events and the state, as in [your first stream](/tutorials/first-stream/).

   <!-- snippet: cart-events -->
   ```cs
   // Events: plain records. No marker interface, no base class.
   public record ItemAdded(string Sku, int Qty);

   public record CheckedOut(DateTimeOffset At);
   ```
   <!-- endSnippet -->

   <!-- snippet: cart-state -->
   ```cs
   // State: Initial and Evolve. Nothing else.
   public record Cart(ImmutableDictionary<string, int> Items, bool IsCheckedOut) : IState<Cart>
   {
       public static Cart Initial { get; } = new(ImmutableDictionary<string, int>.Empty, false);

       public static Cart Evolve(Cart s, object e) => e switch
       {
           ItemAdded x => s with { Items = s.Items.SetItem(x.Sku, s.Items.GetValueOrDefault(x.Sku) + x.Qty) },
           CheckedOut => s with { IsCheckedOut = true },
           _ => s,
       };
   }
   ```
   <!-- endSnippet -->


3. Write an inline projection. It updates your EF Core entity in the append's transaction. You do not call `SaveChanges`; Deedbox does, just before it commits the events.

   <!-- snippet: projection-ef -->
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


4. Register Deedbox. `Run.Inline` means the projection runs in the append's transaction.

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


5. Create the Deedbox tables with an EF Core migration. The tables stay out of your EF model.

   ```sh
   dotnet ef migrations add AddDeedbox
   ```

   Replace the generated `Up` method with the Deedbox script:

   <!-- snippet: schema-ef-migration -->
   ```cs
   // An EF Core migration that creates the Deedbox tables without adding them to your model.
   public partial class AddDeedbox : Migration
   {
       protected override void Up(MigrationBuilder migrationBuilder) =>
           migrationBuilder.Sql(PostgresSchema.Script(fromVersion: 0));   // SqlServerSchema.Script on SQL Server
   }
   ```
   <!-- endSnippet -->


6. Append through your DbContext. `UseDbContext` joins the context's transaction, or opens one and commits it. Pass every context on the same connection that must commit with the events.

   <!-- snippet: transaction-efcore -->
   ```cs
   // Both contexts share one DbConnection. Deedbox enlists them, calls SaveChanges on each,
   // and commits everything at once. With no transaction open, it opens and commits one.
   shop.CartSummaries.Add(new CartSummaryRow("cart-42"));
   await store.UseDbContext(shop, billing).Execute<Cart>("cart-42", cart => CartDecider.Add(cart, "apple", 1));
   ```
   <!-- endSnippet -->


   A context that uses `EnableRetryOnFailure` works here. Deedbox retries nothing inside its transaction. If you want retries, wrap the whole call in the context's execution strategy.

</docs.Steps>

## What changed

- The cart's history is in `deedbox.events`, and its state in `deedbox.streams`.
- `CartSummaries` is still an EF Core table. It changes only through the projection, in the same transaction as the events.
- Every other table and handler in your app is unchanged.

## Next

- [Rebuild a projection](/how-to/rebuild-a-projection/) when you change its logic.
- [Use Dapper or plain ADO.NET](/how-to/use-dapper/) instead of EF Core.
