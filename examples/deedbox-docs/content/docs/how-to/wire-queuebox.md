---
description: Publish events to brokers and webhooks through QueueBox.
sidebarGroup: How-to guides
title: Wire QueueBox
---


This guide shows you how to send events to other systems with [QueueBox](https://github.com/alternayte/queuebox). Deedbox writes a QueueBox outbox row in the append's transaction, so a message exists exactly when its event commits. QueueBox delivers it, with retries, at least once.

1. Run QueueBox against the same database, and let it create its `outbox` table.
2. Add the package:

   ```sh
   dotnet add package Deedbox.QueueBox
   ```

3. Choose the events to publish:

   <!-- snippet: queuebox -->
   ```cs
   services.AddDeedbox(es => es
       .UsePostgres(connStr)
       .Keys(keys => keys.FromEnvironment("DEEDBOX_MASTER_KEY"))
       .Stream<Cart>(s => s.Events<ItemAdded, CheckedOut>())
       .Stream<Manuscript>(s => s.Events<ReviewerInvited, CoAuthorAdded>())
       .UseQueueBox(q => q
           .Publish<CheckedOut>("cart.checked_out")
           // Events with [PersonalData] need a payload you shape, so no personal data leaks by default.
           .Publish<ReviewerInvited>("review.invited", (e, info) => new { e.ManuscriptId, e.ReviewerId })
           // Tell downstream systems to erase too.
           .Publish<SubjectErased>("privacy.subject_erased")));
   ```
   <!-- endSnippet -->


## What each row holds

| Column | Value |
| --- | --- |
| `id` | The event ID. A destination deduplicates on `X-Message-Id`. |
| `topic` | The topic you chose. |
| `key` | The stream ID. It groups one stream's messages; it does not order them. |
| `payload` | The event's JSON, or the payload you shaped. |
| `headers` | `X-Correlation-Id`, `traceparent`, and the event's type, stream and version, plus the headers you add. The stream version is in `x-deedbox-stream-version`. |
| `aggregate_type` | The stream type. |

One stream's messages can reach a destination out of order: the events of one append share an outbox time, and QueueBox publishes a batch concurrently. A receiver that needs order compares `x-deedbox-stream-version` and drops a message older than the one it holds.

An event with `[PersonalData]` needs a payload you shape; start-up fails otherwise ([DBX032](/reference/errors/dbx032/)).

If QueueBox runs with a custom table or column mapping, match it with `UseTable` and `UseColumns`.

## Shape each message

`Publish<TEvent>((e, pending) => new QueueBoxMessage(topic, payload) { Headers = ... })` builds the message for each event. `pending` gives the event ID, type, version, stream ID, time and metadata.

- The topic can differ per event, such as one topic per stream.
- Your headers join the default headers. A header with the name of a default replaces its value, in any letter case. You cannot remove a default.
- Deedbox does not send `EventMetadata.Headers`. Copy `pending.Metadata.Headers` into `Headers` to send them.
- Return `null` to write no message for that event.
- An empty topic, a topic over 255 characters, a header with an empty name or a null value, or an exception in the callback fails the append with [DBX032](/reference/errors/dbx032/). The events and their messages commit together or not at all.

The row ID, the key and `aggregate_type` stay the event ID, the stream ID and the stream type.

## Send CloudEvents

Deedbox has no CloudEvents type. Build the envelope in the message callback.

In structured mode, the payload is the whole CloudEvent:

<!-- snippet: queuebox-cloudevents-structured -->
```cs
// Structured mode: the payload is the whole CloudEvent.
q.Publish<CheckedOut>((e, p) => new QueueBoxMessage("cart.checked_out", new
{
    specversion = "1.0",
    id = p.EventId,
    source = "/shop/carts",
    type = p.EventType,
    subject = p.StreamId,
    time = p.OccurredAt,
    datacontenttype = "application/json",
    data = e,
})
{
    Headers = new Dictionary<string, string> { ["content-type"] = "application/cloudevents+json" },
});
```
<!-- endSnippet -->


In binary mode, the attributes are headers and the payload is the event:

<!-- snippet: queuebox-cloudevents-binary -->
```cs
// Binary mode: the attributes are headers, and the payload is the event.
q.Publish<CheckedOut>((e, p) => new QueueBoxMessage("cart.checked_out", e)
{
    Headers = new Dictionary<string, string>
    {
        ["ce-specversion"] = "1.0",
        ["ce-id"] = p.EventId.ToString(),
        ["ce-source"] = "/shop/carts",
        ["ce-type"] = p.EventType,
        ["ce-subject"] = p.StreamId,
        ["ce-time"] = p.OccurredAt.ToString("O"),
        ["content-type"] = "application/json",
    },
});
```
<!-- endSnippet -->


The HTTP binding names the headers `ce-*`. For Kafka, the binding uses `ce_*`; name the headers the way your destination expects.


## Change a message's contract

A message is a contract with other teams; an event is not. Change them apart.

### Keep the contract when an event changes

When you change an event's shape, the callback gets the new shape, for old events too. Map it to the fields that consumers already read:

<!-- snippet: queuebox-stable-contract -->
```cs
// cart.item_added is at version 3 in the store, and upcasting gives the callback that shape for old events too.
// The message keeps the fields consumers already read, and adds price as a new field they can ignore.
q.Publish<ItemPriced>((e, p) => new QueueBoxMessage("cart.item_added", new { sku = e.Sku, qty = e.Qty, price = e.Price }));
```
<!-- endSnippet -->


Add fields. Do not remove or rename them. A consumer that ignores unknown fields then needs no change.

### Move consumers to a new contract

A breaking change, such as a renamed or removed field, needs a new topic. Deedbox writes one message per event, so the app cannot write the old topic and the new topic in one transaction. Feed the old topic from the new one while consumers move:

1. Start a translator: a consumer that reads the new topic and writes the old contract to the old topic. It keeps each message ID, so consumers still deduplicate.
2. Release the app with the new publication:

   <!-- snippet: queuebox-new-contract -->
   ```cs
   // A breaking change goes to a new topic. From this release, nothing in the app writes the old topic.
   q.Publish<ItemPriced>((e, p) => new QueueBoxMessage("cart.item_added.v2", new
   {
       sku = e.Sku,
       quantity = e.Qty,
       unitPrice = new { amount = e.Price, currency = "EUR" },
   }));
   ```
   <!-- endSnippet -->


3. Move each consumer to the new topic.
4. Confirm that no consumer reads the old topic, from the broker's consumer groups or the QueueBox metrics. Then stop the translator.
