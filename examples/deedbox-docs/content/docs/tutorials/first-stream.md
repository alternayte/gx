---
description: Store a shopping cart as events in about 10 minutes.
order: 1
sidebarGroup: Tutorials
title: Your first stream
---


In this tutorial, you store a shopping cart as a stream of events. You add items, check the cart out, and load its state back. It takes about 10 minutes.

You need the .NET 10 SDK (or .NET 8) and a Postgres or SQL Server database. Docker is the fastest way to get one.

<docs.Steps>

1. Create a web app and add the packages for your database.

   <docs.Tabs sync="db">
   <docs.TabItem label="Postgres">
   ```sh
   dotnet new web -o Shop
   cd Shop
   dotnet add package Deedbox.Postgres
   docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:17-alpine
   ```
   </docs.TabItem>
   <docs.TabItem label="SQL Server">
   ```sh
   dotnet new web -o Shop
   cd Shop
   dotnet add package Deedbox.SqlServer
   docker run -d -p 1433:1433 -e ACCEPT_EULA=Y -e MSSQL_SA_PASSWORD='yourStrong(!)Password' mcr.microsoft.com/mssql/server:2022-latest
   ```
   </docs.TabItem>
   </docs.Tabs>

2. Write the events. An event is a plain record that states what happened.

   <!-- snippet: cart-events -->
```cs
// Events: plain records. No marker interface, no base class.
public record ItemAdded(string Sku, int Qty);

public record CheckedOut(DateTimeOffset At);
```
<!-- endSnippet -->


3. Write the state. `Initial` is the state of a new cart. `Evolve` applies one event.

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


4. Write the decisions. A decision is a pure function: it takes the current state and returns new events. It never writes anything.

   <!-- snippet: cart-decider -->
```cs
// Decisions: pure functions from state to new events.
public static class CartDecider
{
    public static IEnumerable<object> Add(Cart cart, string sku, int qty) =>
        cart.IsCheckedOut
            ? throw new InvalidOperationException("The cart is checked out.")
            : [new ItemAdded(sku, qty)];

    public static IEnumerable<object> CheckOut(Cart cart, DateTimeOffset now) =>
        cart.IsCheckedOut || cart.Items.IsEmpty ? [] : [new CheckedOut(now)];
}
```
<!-- endSnippet -->


5. Register the stream in `Program.cs`. `ApplySchemaOnStartup` creates the Deedbox tables when the app starts.

   <docs.Tabs sync="db">
   <docs.TabItem label="Postgres"><!-- snippet: register-postgres -->
```cs
builder.Services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .ApplySchemaOnStartup()
    .Stream<Cart>(s => s                   // stream type "cart"
        .Events<ItemAdded, CheckedOut>())); // cart.item_added, cart.checked_out
```
<!-- endSnippet -->
</docs.TabItem>
   <docs.TabItem label="SQL Server"><!-- snippet: register-sqlserver -->
```cs
builder.Services.AddDeedbox(es => es
    .UseSqlServer(connStr)
    .ApplySchemaOnStartup()
    .Stream<Cart>(s => s
        .Events<ItemAdded, CheckedOut>()));
```
<!-- endSnippet -->
</docs.TabItem>
   </docs.Tabs>

   Deedbox names the stream type `cart` and the events `cart.item_added` and `cart.checked_out`. These names are stored with every event, so a class rename later does not break stored events.

6. Append events. Get `IEventStore` from dependency injection. `Execute` loads the cart, runs your decision, and appends the new events in one transaction.

   <!-- snippet: write-execute -->
```cs
// Load, decide, evolve and append in one transaction.
var result = await store.Execute<Cart>(cartId, cart => CartDecider.Add(cart, sku, qty));

// result.State is the new state; result.Version the new version; result.Events the appended envelopes.
```
<!-- endSnippet -->


7. Or do each part yourself. `Load` returns the state and the version. `Append` writes only if the stream is still at that version.

   <!-- snippet: write-explicit -->
```cs
var (cart, version) = await store.Load<Cart>(cartId);
var events = CartDecider.CheckOut(cart, now).ToList();
if (events.Count > 0)
    await store.Append(cartId, ExpectedVersion.Exact(version), events);
```
<!-- endSnippet -->


</docs.Steps>

## What you have now

- A `deedbox` schema with a `streams` table and an `events` table.
- One stream, `cart-1`, with its events in order, and its current state stored next to it.
- No mediator, no base class and no changes to the rest of your app.

## Next

- [Add a projection](/how-to/add-a-projection/) to build a read model.
- [Add Deedbox to an existing EF Core app](/tutorials/existing-ef-core-app/).
- Read [streams and state](/concepts/streams-and-state/) to see what Deedbox stores.
