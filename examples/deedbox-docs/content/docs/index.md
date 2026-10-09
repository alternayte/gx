---
description: Event-source part of your app. Postgres or SQL Server. EF Core, Dapper, or neither.
template: splash
title: Deedbox
---


Deedbox is an MIT-licensed .NET library. It stores the events of one area of your app, such as orders or manuscripts, in your existing Postgres or SQL Server database. The rest of your app stays as it is.

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

<!-- snippet: write-execute -->
```cs
// Load, decide, evolve and append in one transaction.
var result = await store.Execute<Cart>(cartId, cart => CartDecider.Add(cart, sku, qty));

// result.State is the new state; result.Version the new version; result.Events the appended envelopes.
```
<!-- endSnippet -->


<docs.CardGrid>
  <docs.Card title="Your app stays yours" icon="setting">
    Deedbox tables live in their own schema. Appends share your connection and transaction. There is no mediator, no document database and no base class for your events.
  </docs.Card>
  <docs.Card title="Correct under load" icon="approve-check">
    No event is ever skipped. Global order is commit order. A torture suite checks this on both databases, with killed sessions and competing instances.
  </docs.Card>
  <docs.Card title="Erasure built in" icon="warning">
    Mark personal data with an attribute. Erasing a person makes their data unreadable everywhere, even inside long-lived shared streams.
  </docs.Card>
  <docs.Card title="Operable" icon="list-format">
    A CLI, an admin API, metrics, traces and health checks that stay healthy during a rebuild.
  </docs.Card>
</docs.CardGrid>
