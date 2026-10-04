---
description: Why decisions are pure functions, and how Execute uses that.
order: 2
sidebarGroup: Concepts
title: Decide and evolve
---


This page explains the two functions every stream needs, and how Deedbox runs them.

- **Evolve** applies one event to a state. It is part of the state type, and it never fails.
- **Decide** takes the current state and a command, and returns new events. It reads nothing and writes nothing.

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


## Execute locks, decides and appends

<!-- snippet: write-execute -->
```cs
// Load, decide, evolve and append in one transaction.
var result = await store.Execute<Cart>(cartId, cart => CartDecider.Add(cart, sku, qty));

// result.State is the new state; result.Version the new version; result.Events the appended envelopes.
```
<!-- endSnippet -->


`Execute` locks the stream, loads its state, runs your decision, and appends the events, in one transaction. Because the stream stays locked from load to commit, two writers of one stream take turns instead of conflicting. When two writers create the same new stream at once, the loser runs its decision again; `ExecuteRetries` sets how often (3 by default).

## The explicit form works with any error style

<!-- snippet: write-explicit -->
```cs
var (cart, version) = await store.Load<Cart>(cartId);
var events = CartDecider.CheckOut(cart, now).ToList();
if (events.Count > 0)
    await store.Append(cartId, ExpectedVersion.Exact(version), events);
```
<!-- endSnippet -->


`Append` checks the expected version and writes only if it matches. Deedbox has no `Result` type and no opinion on errors: your decision can return a result, throw, or return no events.

<!-- snippet: write-conflict -->
```cs
try
{
    await store.Append(cartId, ExpectedVersion.NoStream, [new ItemAdded("apple", 1)]);
}
catch (ConcurrencyException ex)
{
    // ex.Expected is NoStream; ex.Actual is the version the stream is at.
    Console.WriteLine($"Cart {ex.StreamId} already exists at version {ex.Actual}.");
}
```
<!-- endSnippet -->


`ExpectedVersion` is `NoStream`, `Any` or `Exact(n)`. `Exact(0)` is the same as `NoStream`.
