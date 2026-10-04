---
description: Reset a projection and replay every event through it, without downtime.
sidebarGroup: How-to guides
title: Rebuild a projection
---


This guide shows you how to rebuild a projection after you change its logic.

1. Make sure the projection overrides `ResetAsync`. Without it, the rebuild job fails with [DBX023](/reference/errors/dbx023/).
2. Deploy the new projection code.
3. Queue the rebuild from the CLI:

   ```sh
   deedbox rebuild cart_summary --provider postgres --connection "$DEEDBOX_CONNECTION" --wait
   ```

   Or from code:

   <!-- snippet: admin-api -->
```cs
var status = await admin.GetStatusAsync();
foreach (var consumer in status.Consumers)
    Console.WriteLine($"{consumer.Name}: {consumer.Status}, {consumer.Lag} behind");

var rebuild = await admin.RebuildAsync("cart_summary");
var skip = await admin.SkipAsync("cart_totals", stalledEventId);
var job = await admin.GetJobAsync(rebuild);
```
<!-- endSnippet -->


## What happens

1. A running app instance takes the job. It calls `ResetAsync` and sets the projection to `rebuilding`, in one transaction.
2. The background runner replays every event from position 0.
3. For an inline projection, appends skip the projection while it rebuilds. When the replay is within one batch of the head, the runner locks the position counter, applies the last events, and sets the projection back to `running`. Appends apply it inline again from then on.
4. For an async projection, the runner sets it back to `running` when it reads to the end.

During the rebuild, the read model is empty or partial. The health check reports `rebuilding` as healthy, so Kubernetes does not restart the app. To keep the old read model serving reads during the change, [replace it without downtime](/how-to/replace-a-read-model/) instead.

If appends arrive faster than the replay, the runner switches over anyway after 20 polls without progress. Appends then wait while it applies the rest.
