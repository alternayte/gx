---
description: Call a service before a decision for the data it needs, or after it for a side effect, and handle each failure.
sidebarGroup: How-to guides
title: Call an external service
---


This guide shows you where to call an external service, such as a pricing API or a payment provider, when you write events.

## Do not call a service in a decision

A decision is a function from state to events. It is synchronous, and it gets no services. Deedbox relies on this:

- `Execute` holds the stream's row lock and a transaction while the decision runs. A network call in a decision makes every other writer of that stream wait for it.
- `Execute` runs a decision again when it loses a race to create the stream. A call in the decision then runs again too.
- The call can succeed and the commit can fail. The change in the other system then stays, and no event records it.

When a decision throws, `Execute` rolls back its transaction and passes the exception to you. It writes no events, no inline projection rows and no QueueBox messages.

Put each call before or after the decision, by what the call does:

| The call | Where | Example |
| --- | --- | --- |
| Reads data that the decision needs | Before `Execute` | A price, a stock level, an address check |
| Changes something in another system | After the decision, in a subscription | A payment, an email, an account in another service |

## Call before the decision for data

<!-- snippet: external-call-before -->
```cs
// Call the service first. If it throws, Execute does not run and nothing is written.
var quote = await pricing.QuoteAsync(skus, ct);

// The decision gets the quote as a value, so it stays pure and safe to run again.
await store.Execute<Order>(orderId, order => OrderDecider.Place(order, quote), ct);
```
<!-- endSnippet -->


If the service fails, you do not call `Execute`, and nothing is written. The value can be out of date when the transaction commits. When that matters, the decision checks the value against the state.

## Call after the decision for side effects

The decision records the request as an event. A subscription makes the call, then records the result as another event.

<!-- snippet: external-order -->
```cs
public record OrderPlaced(decimal Total);

public record PaymentRequested(decimal Amount);

public record PaymentCaptured(string ChargeId);

public record PaymentDeclined(string Reason);

public record Order(decimal Total, string Status) : IState<Order>
{
    public static Order Initial { get; } = new(0m, "new");

    public static Order Evolve(Order s, object e) => e switch
    {
        OrderPlaced x => s with { Total = x.Total, Status = "awaiting_payment" },
        PaymentCaptured => s with { Status = "paid" },
        PaymentDeclined => s with { Status = "declined" },
        _ => s,
    };
}

public static class OrderDecider
{
    public static IEnumerable<object> Place(Order order, Quote quote) =>
        order.Status != "new"
            ? throw new InvalidOperationException("The order is already placed.")
            : [new OrderPlaced(quote.Total), new PaymentRequested(quote.Total)];

    // A redelivered event finds the payment already recorded, and records nothing.
    public static IEnumerable<object> RecordPayment(Order order, ChargeResult result) =>
        order.Status != "awaiting_payment" ? []
        : result.ChargeId is { } id ? [new PaymentCaptured(id)]
        : [new PaymentDeclined(result.DeclineReason ?? "declined")];
}
```
<!-- endSnippet -->


<!-- snippet: external-call-after -->
```cs
public sealed class ChargePayment : Subscription
{
    public ChargePayment(IPayments payments) =>
        On<PaymentRequested>(async (e, ctx) =>
        {
            // The event ID is the idempotency key, so a retried event does not charge twice.
            var result = await payments.ChargeAsync(e.Amount, ctx.Envelope.EventId.ToString(), ctx.CancellationToken);

            // A declined card is a result, so the order records it. A timeout throws, and the event is retried.
            var store = ctx.Services.GetRequiredService<IEventStore>();
            await store.Execute<Order>(ctx.Envelope.StreamId, order => OrderDecider.RecordPayment(order, result), ctx.CancellationToken);
        });
}
```
<!-- endSnippet -->


<!-- snippet: external-call-register -->
```cs
services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Stream<Order>(s => s.Events<OrderPlaced, PaymentRequested, PaymentCaptured, PaymentDeclined>())
    .Subscription<ChargePayment>("charge-payment"));
```
<!-- endSnippet -->


- A subscription gets each event at least once. Give the event ID to the service as its idempotency key, so a retry does not charge twice.
- The store from `ctx.Services` appends with the event's tenant and correlation ID, and records the event as the cause.
- The subscription can append its result and stop before its checkpoint moves. The event then arrives again. `RecordPayment` finds the order already paid and returns no events.

A subscription changes other streams in the same way. Deedbox appends to one stream per transaction, so a change to a second stream follows as its own event; see [known limits](/concepts/known-limits/).

When the other system is your own service and reads messages, publish the event instead of calling the service; see [wire QueueBox](/how-to/wire-queuebox/).

## Handle each failure

- **The service returns a result that the business expects**, such as a declined card: record it as an event. Do not throw. The order then shows why it stopped, and the subscription moves on.
- **The service is down or times out:** throw. The runner retries the event after `RetryDelay`, and doubles the delay on each retry. After `HandlerRetries` retries, it stalls the subscription.
- **The subscription stalls:** it handles no later events of its own. Other consumers keep running. The runner retries the event every 5 minutes, and the subscription runs again once the service is back. To give up on the event, skip it; see [poison event](/operations/poison-event/).

With the defaults, the subscription stalls after about 30 seconds of failures, and the health check then reports it. To stall later, raise `HandlerRetries`; the runner settings apply to every consumer, and [configuration](/reference/configuration/) lists them.
