---
description: Turn an email or a login into a keyed subject ID, and erase a person by identity.
sidebarGroup: How-to guides
title: Use pseudonymous subject IDs
---


This guide shows you how to store subject IDs that do not show who a person is, and how to erase a person by their real identity.

Subject IDs and metadata are plain text. An email or a login in a subject ID is personal data in every table, log and backup that holds it. A hash of an email does not help: anyone with a list of emails can hash the list and compare.

Deedbox makes a keyed subject ID instead. For an identity such as `github:alice`, it computes `person:` plus 26 base32 characters of HMAC-SHA256 under a secret key. Without the secret, nobody can link the ID to the person.

## Choose a key mode and a prefix

The master key wraps each pseudonym secret, so you need a key mode. See [erase a person](/how-to/erase-a-person/) for the modes.

<!-- snippet: pseudonyms-register -->
```cs
services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Keys(keys => keys.FromEnvironment("DEEDBOX_MASTER_KEY"))
    .PseudonymPrefix("person:")  // the default; set it once
    .Stream<Session>(s => s.Events<CorrectionRecorded>()));
```
<!-- endSnippet -->


The prefix is `person:` by default. Set it once. A period keeps the prefix it was created with, and a different prefix fails with [DBX037](/reference/errors/dbx037/).

## Mark the subject

<!-- snippet: pseudonym-event -->
```cs
public record CorrectionRecorded(
    [property: DataSubject] string Author,   // a pseudonymous subject ID, never the login
    [property: PersonalData] string? Text);  // free text can still name a person
```
<!-- endSnippet -->


The subject ID is the pseudonymous ID. Free text can still name a person, so keep `[PersonalData]` on it.

## Compute the subject ID when you write

<!-- snippet: pseudonyms-write -->
```cs
// One secret per quarter: the same person gets a new subject ID each quarter.
var author = await pseudonyms.SubjectForAsync($"github:{login}", PseudonymPeriod.Quarter(clock.GetUtcNow()));
await store.Append(sessionId, ExpectedVersion.Any, [new CorrectionRecorded(author, text)]);
```
<!-- endSnippet -->


- Pass a canonical, namespaced identity, such as `email:alice@example.com` or `github:alice`. Deedbox does not change case and does not resolve aliases. If one person has several identities, map them to one identity before the call.
- The period is an ID that you choose. `PseudonymPeriod.Quarter` gives `2026-Q3` and `PseudonymPeriod.Month` gives `2026-09`, both in UTC.
- Each (tenant, period) has its own secret. The same person gets a different subject ID in each period, so nobody can follow them across periods by ID alone.
- For subject IDs that never change, pass one fixed period ID, such as `all`.
- Deedbox creates a period's secret on first use. Every instance then computes the same ID.

Reads, rebuilds, projections and erasure by subject ID never need the secret. Only the call that computes a subject ID, and erasure by identity, need it.

## Erase a person by identity

<!-- snippet: pseudonyms-erase -->
```cs
// Erases the person's subject in every period whose secret still exists.
var jobIds = await pseudonyms.EraseIdentityAsync("github:alice");
```
<!-- endSnippet -->


1. Deedbox computes the person's subject ID in every period whose secret still exists.
2. It deletes the subject key of each of those subjects in one transaction. Their personal data reads as erased at once.
3. It queues one erasure job per period in the same transaction, as [erase a person](/how-to/erase-a-person/) describes.

`IEventStoreAdmin.EraseIdentityAsync(identity, tenantId)` does the same for a named tenant. The tenant is required; pass `""` when the app has no tenants. It returns an `ErasureResult` with the job IDs and the number of keys deleted. From the CLI:

```sh
deedbox erase --identity github:alice --tenant acme --master-key env:DEEDBOX_MASTER_KEY --wait
```

The CLI unwraps the secrets itself, so `--master-key` names the master key the app uses: `database`, `env:<VARIABLE>` or `azure:<key URL>`. Deedbox never stores, logs or prints the identity.

## Destroy an old period's secret

When the app no longer writes in a period, destroy its secret. After that, nobody can link that period's subject IDs to a person, not even an admin with a list of identities.

<!-- snippet: pseudonyms-destroy -->
```cs
// The app no longer writes in 2026-Q1. Nobody can link its subject IDs to a person again.
var destroyed = await admin.DestroyPseudonymPeriodAsync("2026-Q1", tenantId: "acme");
```
<!-- endSnippet -->


The tenant is required here too; pass `""` when the app has no tenants.

From the CLI:

```sh
deedbox pseudonyms destroy 2026-Q1 --tenant acme --yes
```

- The period stays closed. A later call for it fails with [DBX036](/reference/errors/dbx036/), so a late write cannot give the same person a second ID in that period.
- A `pseudonyms_destroyed` row in the jobs table records each destroy. `deedbox status` lists it.
- Personal data stays readable, and erasure by subject ID still works. Erasure by identity skips the period, because its subject IDs cannot be computed.

Shredding a tenant deletes all its pseudonym secrets too. Re-wrapping or rotating the master key does not change any subject ID.

## Know the limits

- A person who holds a period's secret and a list of candidate identities can re-identify that period's subjects. The master key protects the secrets, so control who can use the master key.
- Pseudonymized data is still personal data under the GDPR and the Swiss data protection act.
- The pseudonymizer protects IDs, not content. Text that names a person needs `[PersonalData]`.
- Backups taken before a destroy keep the wrapped secret until they age out.
