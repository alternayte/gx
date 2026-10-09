---
description: Add, remove or change fields of an event with upcasters.
sidebarGroup: How-to guides
title: Change an event's shape
---


This guide shows you how to change an event's fields and keep old events readable.

An event's shape version is stored with it, in `event_version`. The name does not change. When you change the shape, raise the version and add an upcaster for each step from the old versions.

## Edit the JSON for small changes

<!-- snippet: upcast-json -->
```cs
services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Stream<Cart>(s => s
        .Event<ItemAdded>(version: 2, up => up
            .From(1, json => json["qty"] ??= 1))  // version 1 had no quantity
        .Event<CheckedOut>()));
```
<!-- endSnippet -->


`From(1, ...)` receives the stored JSON of version 1 and changes it into version 2. It runs after decryption and before deserialization.

## Convert a kept record for a real reshape

<!-- snippet: evolution-records -->
```cs
public record LineAdded(string Sku, int Qty);             // was called ItemAdded

public record ItemAddedV2(string Sku, int Qty);           // the old shape, kept for the typed upcaster

public record ItemPriced(string Sku, int Qty, decimal Price);
```
<!-- endSnippet -->

<!-- snippet: upcast-typed -->
```cs
services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Stream<Cart>(s => s
        .Event<ItemPriced>(version: 3, up => up
            .Name("cart.item_added")
            .From(1, json => json["qty"] ??= 1)
            .Upcast<ItemAddedV2, ItemPriced>(old => new ItemPriced(old.Sku, old.Qty, 0m)))
        .Event<CheckedOut>()));
```
<!-- endSnippet -->


Steps run in order: version 1 goes through the JSON step to version 2, then the typed step converts version 2 to the current record. A typed step is always the last one.

## What needs a new version

| Change | New version needed |
| --- | --- |
| Add a nullable property | No |
| Add a property that cannot be null | Yes, with a default in the upcaster |
| Remove or rename a property | Yes |
| Change a property's type | Yes |

A property of a polymorphic type, such as a base type with `[JsonDerivedType]`, works on .NET 9 and later. On .NET 8 with Postgres, start-up fails with [DBX039](/reference/errors/dbx039/).

Start-up fails when a version has no upcaster for some step ([DBX018](/reference/errors/dbx018/)). The [lockfile](/how-to/test-deciders/#pin-event-contracts) fails a test when a shape changes without a new version.
