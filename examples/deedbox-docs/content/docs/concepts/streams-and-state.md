---
description: What Deedbox stores for each stream, and how it names things.
order: 1
sidebarGroup: Concepts
title: Streams and state
---


This page explains what a stream is, what Deedbox stores for it, and how names stay stable.

## A stream is one aggregate's history

A stream holds the events of one thing, such as one cart. Its identity is (tenant, stream ID). Its events have versions 1, 2, 3 and so on, with no gaps. The stream's state is what you get when you apply every event, in order, to the initial state.

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


## Names are stored, not derived at read time

| Thing | Convention | Override |
| --- | --- | --- |
| Stream type | `Cart` becomes `cart` | `.Stream<Cart>("shopping_cart", ...)` |
| Event type | `cart.item_added` | `.Event<ItemAdded>(name: "...")` |
| Old names | none | `.Alias("...")` |
| Shape version | 1 | `.Event<T>(version: 2, ...)` |

<!-- snippet: register-names -->
```cs
builder.Services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Stream<Cart>("shopping_cart", s => s                   // stored as "shopping_cart"
        .Event<ItemAdded>(name: "shopping_cart.line_added") // an explicit event name
        .Event<CheckedOut>()));                             // shopping_cart.checked_out
```
<!-- endSnippet -->


One CLR type maps to one stored name. Registering it twice fails at start-up. At start-up, Deedbox also checks that every stored event name maps to a registered event.

## Stream IDs are strings

<!-- snippet: stream-ids -->
```cs
var fromGuid = StreamId.From(Guid.NewGuid());   // "0f8fad5b-d9cb-469f-a165-70867728950e"

var ns = Guid.Parse("a1b2c3d4-e5f6-7890-abcd-ef1234567890");
var forPair = StreamId.Deterministic(ns, userId.ToString(), titleId.ToString()); // the same pair gives the same ID
```
<!-- endSnippet -->


A stream ID has 1 to 200 characters and no leading or trailing white space. IDs compare case-sensitively on both databases.

## State is stored next to the events

By default, every append also stores the new state, so a load reads one row. The stored state carries a state version. When you change the state record, raise the state version: each stream's state is then rebuilt from its events on its next load.

<!-- snippet: register-snapshots -->
```cs
builder.Services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Stream<Cart>(s => s
        .Events<ItemAdded, CheckedOut>()
        .StateVersion(2)                         // raise it when you change the Cart record
        .Snapshots(SnapshotPolicy.Every(50))));  // or EveryAppend (default) or Never
```
<!-- endSnippet -->


For a stream type with personal data, the stored state is encrypted with the tenant's key.
