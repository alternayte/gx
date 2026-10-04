---
description: Test decisions with Given/When/Then, and pin event contracts with a lockfile.
sidebarGroup: How-to guides
title: Test deciders and event contracts
---


This guide shows you how to test your decisions without a database, and how to stop a change to an event from breaking stored events.

```sh
dotnet add package Deedbox.Testing
```

## Test decisions

<!-- snippet: decider-tests -->
```cs
public class CartDeciderTests
{
    private static readonly DateTimeOffset Now = new(2026, 9, 24, 12, 0, 0, TimeSpan.Zero);

    [Fact]
    public void A_cart_with_items_checks_out() =>
        Decider.Given<Cart>(new ItemAdded("apple", 2))
            .When(cart => CartDecider.CheckOut(cart, Now))
            .Then(new CheckedOut(Now));

    [Fact]
    public void An_empty_cart_does_not_check_out() =>
        Decider.Given<Cart>()
            .When(cart => CartDecider.CheckOut(cart, Now))
            .ThenNothing();

    [Fact]
    public void A_checked_out_cart_takes_no_items() =>
        Decider.Given<Cart>(new ItemAdded("apple", 2), new CheckedOut(Now))
            .When(cart => CartDecider.Add(cart, "pear", 1))
            .ThenThrows<InvalidOperationException>();
}
```
<!-- endSnippet -->


`Given` folds past events through your state's `Evolve`. `When` runs the decision. `Then` compares the events by type and by their JSON, so records that hold collections compare by content. The helpers throw `DeciderAssertionException`, so they work with any test framework.

## Pin event contracts

<!-- snippet: lockfile-test -->
```cs
public class EventContractTests
{
    [Fact]
    public void Event_contracts_are_stable() =>
        EventContracts.Verify(es => es.Stream<Cart>(s => s.Events<ItemAdded, CheckedOut>()), "events.lock");
}
```
<!-- endSnippet -->


The first run writes `events.lock` next to the test file and fails once; commit the file. From then on:

- A new event, alias, version or nullable property updates the file, and the test passes. Commit the change.
- A removed name, a changed type, a removed property, a removed `[PersonalData]` marker, or a new non-nullable property without a new version fails the test with the fix.
- Under CI (`CI=true`), any difference fails, so a stale lockfile cannot pass.

Compare two lockfiles in a pull request with `deedbox lockfile diff main.lock events.lock`.
