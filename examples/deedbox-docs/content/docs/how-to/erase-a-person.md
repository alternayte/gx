---
description: Mark personal data, then erase a data subject everywhere.
sidebarGroup: How-to guides
title: Erase a person
---


This guide shows you how to store personal data so that you can erase one person later, even when their data sits inside streams you must keep.

## Mark the data

<!-- snippet: personal-data-event -->
```cs
public record ReviewerInvited(
    string ManuscriptId,
    [property: DataSubject] string ReviewerId,     // whose data this is
    [property: PersonalData] string ReviewerName,  // encrypted under ReviewerId's key
    [property: PersonalData] string? ReviewerEmail);
```
<!-- endSnippet -->


`[DataSubject]` marks whose data the event carries. Each `[PersonalData]` field is encrypted under that subject's own key. A personal-data property must be a string or nullable, because it reads as null after erasure.

Use person IDs, not role IDs, as subjects. One erasure then covers every role a person has.

<!-- snippet: personal-data-several -->
```cs
// Several subjects in one event: name each field's subject property.
public record CoAuthorAdded(
    string ManuscriptId,
    string AuthorId,
    [property: PersonalData(Subject = "AuthorId")] string AuthorName,
    string EditorId,
    [property: PersonalData(Subject = "EditorId")] string? EditorNote);
```
<!-- endSnippet -->


## Choose a key mode

Start-up fails until you choose one ([DBX025](/reference/errors/dbx025/)).

<!-- snippet: keys-database -->
```cs
services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Keys(keys => keys.StoreInDatabase())
    .Stream<Manuscript>(s => s.Events<ReviewerInvited, CoAuthorAdded>()));
```
<!-- endSnippet -->


`StoreInDatabase` keeps the master key in the same database. Erasure works, but a copy of the database exposes personal data. Move to a key ring or Azure Key Vault before production:

<!-- snippet: keys-environment -->
```cs
// DEEDBOX_MASTER_KEY holds a key ring: v2:<base64 of 32 random bytes>,v1:<older key>
services.AddDeedbox(es => es
    .UsePostgres(connStr)
    .Keys(keys => keys
        .FromEnvironment("DEEDBOX_MASTER_KEY")
        .RedactWith("[erased]"))
    .Stream<Manuscript>(s => s.Events<ReviewerInvited, CoAuthorAdded>()));
```
<!-- endSnippet -->


See [rotate keys](/how-to/rotate-keys/) to move between modes.

## Erase the subject

<!-- snippet: erase-subject -->
```cs
// Their data reads as erased as soon as this returns; the job finishes the rest.
var jobId = await erasure.EraseSubjectAsync("person:8421");
```
<!-- endSnippet -->


1. Deedbox deletes the subject's key and clears the stored state of every stream with their data, in one transaction. From then on, every read returns their fields as null (or the `RedactWith` placeholder).
2. A job appends a `SubjectErased` event to each of those streams and stores rebuilt state. Handle `SubjectErased` in your projections to scrub what they stored.
3. The job resumes after a crash.

From the CLI: `deedbox erase person:8421 --tenant acme`.

## Know the limits

- Backups taken before an erasure still hold the subject's key until they age out. Set your backup retention with this in mind.
- Metadata and subject IDs are not encrypted. Use pseudonymous IDs such as `person:8421`. [Use pseudonymous subject IDs](/how-to/use-pseudonymous-ids/) shows how Deedbox makes them from an email or a login.
- Only top-level properties are encrypted.
- Data written about the subject after the erasure uses a new key and stays readable.
