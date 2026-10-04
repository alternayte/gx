---
description: Keep each tenant's streams apart in shared tables.
sidebarGroup: How-to guides
title: Use tenants
---


This guide shows you how to scope streams to tenants.

Every stream belongs to a tenant: its identity is the pair (tenant, stream ID). The default tenant is empty. Set the tenant once per request on the scoped `DeedboxContext`; every store in that scope loads and appends in it.

<!-- snippet: metadata-middleware -->
```cs
// Set the tenant and metadata once per request; every store in the request scope uses them.
app.Use(async (http, next) =>
{
    var deedbox = http.RequestServices.GetRequiredService<DeedboxContext>();
    deedbox.TenantId = http.Request.Headers["X-Tenant"].ToString();
    deedbox.Metadata = new EventMetadata
    {
        CorrelationId = http.TraceIdentifier,
        Actor = http.User.Identity?.Name is { } user ? $"user:{user}" : null,
    };
    await next(http);
});
```
<!-- endSnippet -->


- The same stream ID in two tenants is two streams.
- Async projections see all tenants, in one global order. `ctx.TenantId` names each event's tenant.
- Each tenant has its own encryption key, so you can [shred a whole tenant](/how-to/rotate-keys/#shred-a-whole-tenant).
- A background job sets the tenant in its own scope before it resolves a store.

Tenant IDs are at most 100 characters with no leading or trailing white space.
