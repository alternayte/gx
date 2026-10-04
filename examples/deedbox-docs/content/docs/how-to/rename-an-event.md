---
description: Rename an event class without breaking stored events.
sidebarGroup: How-to guides
title: Rename an event
---


This guide shows you how to rename an event class and keep its stored events readable.

Every stored event keeps the name it was written under, such as `cart.item_added`. A rename changes the conventional name, so stored events have no class to read into. Deedbox fails at start-up in that case, with [DBX016](/reference/errors/dbx016/), and names the likely rename.

## Keep the old name as an alias

<!-- snippet: evolution-records -->
```cs
public record LineAdded(string Sku, int Qty);             // was called ItemAdded

public record ItemAddedV2(string Sku, int Qty);           // the old shape, kept for the typed upcaster

public record ItemPriced(string Sku, int Qty, decimal Price);
```
<!-- endSnippet -->

<!-- snippet: rename-alias -->
```cs
services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Stream<Cart>(s => s
        .Event<LineAdded>(e => e.Alias("cart.item_added")) // stored events keep their old name
        .Event<CheckedOut>()));
```
<!-- endSnippet -->


New events are stored under the new name, `cart.line_added`. Stored events under `cart.item_added` read as `LineAdded`.

To keep the old name for new events too, set it explicitly instead: `.Event<LineAdded>(name: "cart.item_added")`.

## Let CI catch it first

The [event contract lockfile](/how-to/test-deciders/#pin-event-contracts) fails a test when a name disappears without an alias.
