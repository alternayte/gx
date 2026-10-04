---
description: Build a changed read model in new tables while the old one serves reads, then switch.
sidebarGroup: How-to guides
title: Replace a read model without downtime
---


This guide shows you how to change a read model's tables or shape while the old read model keeps serving reads. An [in-place rebuild](/how-to/rebuild-a-projection/) is simpler, but the read model is empty or partial until it finishes. For a large read model, that gap can be long.

The method is often called a blue/green rebuild: the old projection is blue, the new one is green. Deedbox builds it from the events, so the new read model gets every change since the first event.

1. Write the new projection as a new class that writes to new tables, such as `cart_summary_v2`. Create the tables.

2. Register it under a new name, next to the old one. Keep the old one as it is: it serves reads until the switch.

   <!-- snippet: blue-green-register -->
```cs
services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Stream<Cart>(s => s.Events<ItemAdded, CheckedOut>())
    .Projection<CartSummaryProjection>("cart_summary", Run.Inline)        // serves reads until the switch
    .Projection<CartSummaryV2Projection>("cart_summary_v2", Run.Async));  // fills the new tables from the first event
```
<!-- endSnippet -->


   The new one can be async or inline. A new inline projection stays in catch-up until no instance of the old version is live, so it misses nothing during a rolling deploy; see [inline or async projections](/concepts/inline-or-async/#deploy-inline-projections-safely).

3. Deploy. The runner applies every earlier event to the new projection, then keeps it current. Watch it with `deedbox status`, or from code:

   <!-- snippet: blue-green-ready -->
```cs
var status = await admin.GetStatusAsync();
var next = status.Consumers.Single(c => c.Name == "cart_summary_v2");
var ready = next.Status == "running" && next.Lag == 0;  // caught up with every event
```
<!-- endSnippet -->


   The health check stays healthy while the new projection catches up.

4. When the new projection is caught up, switch the reads to the new tables. Use a configuration flag or a deploy. Both read models stay current until the next step, so you can switch back.

5. Remove the old projection's registration, and deploy.

6. Retire the old projection, then drop its tables:

   ```sh
   deedbox retire cart_summary --provider postgres --connection "$DEEDBOX_CONNECTION"
   ```

   Or from code, with `IEventStoreAdmin.RetireAsync("cart_summary")`. Retiring refuses with [DBX035](/reference/errors/dbx035/) while a live instance still registers the name, and names those instances. A retired projection keeps its checkpoint, as `retired`. If an old version with the name starts again, as in a rollback, the projection stays idle and the health check reports it as degraded. `RebuildAsync("cart_summary")` brings it back.

## Change the mode later

A projection's mode is part of its checkpoint, and a projection whose mode changes stalls until it is rebuilt. To change the mode of the new read model without a gap in its reads, repeat this guide with a third name in the other mode.
