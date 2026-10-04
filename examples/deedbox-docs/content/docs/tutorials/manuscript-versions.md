---
description: Model submission, review rounds, acceptance, publication and corrections, and serve the latest version and the full history over REST.
order: 3
sidebarGroup: Tutorials
title: Version a manuscript
---


In this tutorial, you build the editorial core of a journal. A manuscript goes through review rounds, acceptance, publication and a correction. Each version that a reviewer or a reader sees stays frozen. A REST API lists manuscripts, and serves the latest version and the full history.

Do [your first stream](/tutorials/first-stream/) first. You need a Postgres database and these packages:

```sh
dotnet add package Deedbox.Postgres
dotnet add package Deedbox.EntityFrameworkCore
dotnet add package Npgsql.EntityFrameworkCore.PostgreSQL
```

## Three kinds of version

Keep these apart, or the design goes wrong:

- **Stream version**: Deedbox counts every event. It serves concurrency only. Never show it to a reviewer.
- **Event shape version**: `Event<T>(2, ...)` and its upcasters, for when an event's JSON changes. See [change an event's shape](/how-to/change-an-event-shape/).
- **Manuscript version**: a frozen copy of the content that someone submitted, reviewed or published. This tutorial is about this kind.

The names come from scholarly publishing standards:

| Standard | What it says | Where it shows up here |
| --- | --- | --- |
| [NISO Journal Article Versions](https://www.niso.org/publications/niso-rp-8-2008-jav) | Names the stages: Submitted Manuscript Under Review, Accepted Manuscript, Version of Record, Corrected Version of Record, and others. | `Stage` |
| [Crossref versioning](https://www.crossref.org/documentation/principles-practices/best-practices/versioning/) | The accepted manuscript and the Version of Record share one DOI. A correction does not change the article; a separate notice with its own DOI explains it. | `Published`, `UpdateIssued` |
| [NISO CREC](https://www.niso.org/publications/rp-45-2024-crec) | A retracted article stays findable and marked. | `Retract` deletes nothing |

<docs.Steps>

1. Write the vocabulary and the events.

   <!-- snippet: publishing-vocabulary -->
```cs
// Version stages from NISO Journal Article Versions (RP-8-2008).
public enum Stage { SubmittedUnderReview, AcceptedManuscript, VersionOfRecord, CorrectedVersionOfRecord }

public enum Status { Draft, UnderReview, InRevision, Accepted, Rejected, Published, Retracted }

public enum Decision { MinorRevision, MajorRevision, Accept, Reject }

// Crossref Crossmark update types, as NISO CREC uses them.
public enum UpdateType { Correction, ExpressionOfConcern, Retraction }
```
<!-- endSnippet -->


   <!-- snippet: publishing-events -->
```cs
// A section's text lives in blob storage; events hold its content hash.
public record SectionRef(string SectionId, string Heading, string ContentHash);

public record ManuscriptStarted(string Title);

public record SectionRevised(string SectionId, string Heading, string ContentHash);

// Flat on purpose: Deedbox encrypts top-level [PersonalData] properties only.
// PersonId names the person across every manuscript they write; the affiliation belongs to this authorship.
public record AuthorAdded(
    [property: DataSubject] string PersonId,
    [property: PersonalData] string Name,
    [property: PersonalData] string? Orcid,
    string Affiliation,
    bool Corresponding);

// A version is a frozen list of sections. Nothing changes it later.
public record VersionFrozen(int Number, Stage Stage, IReadOnlyList<SectionRef> Sections, int? BasedOn, string? Reason);

public record ReviewRoundOpened(int Round, int Version);

public record DecisionMade(int Round, Decision Decision);

public record Published(int Version, string Doi);

public record UpdateIssued(UpdateType Type, string NoticeDoi, int? Version);
```
<!-- endSnippet -->


   Two rules shape these events. A section's text stays out of the event: the event holds a content hash, and blob storage holds the text. `AuthorAdded` is flat, because Deedbox encrypts top-level `[PersonalData]` properties only.

   `PersonId` names the person across every manuscript they write, so one person has one profile. The affiliation and the corresponding-author flag belong to this authorship: a person can write under a different affiliation on each manuscript.

   A version is a `VersionFrozen` event. It lists the sections as they are at that moment, and nothing changes it later.

2. Write the state. It holds only what the decisions need: the working copy, the sections of the latest version, and the status. The version history does not go in the state; the read model keeps it.

   <!-- snippet: publishing-state -->
```cs
// The state holds only what decisions need. The version history lives in the read model.
public record Manuscript(
    string? Title,
    Status Status,
    ImmutableList<SectionRef> Draft,   // the working copy that authors edit
    ImmutableList<SectionRef> Frozen,  // the sections of the latest version
    ImmutableList<string> Authors,     // person IDs, in byline order
    int LatestVersion,
    int Round) : IState<Manuscript>
{
    public static Manuscript Initial { get; } = new(null, Status.Draft, [], [], [], 0, 0);

    public static Manuscript Evolve(Manuscript m, object e) => e switch
    {
        ManuscriptStarted s => m with { Title = s.Title },
        SectionRevised r => m with { Draft = Put(m.Draft, new(r.SectionId, r.Heading, r.ContentHash)) },
        AuthorAdded a => m with { Authors = m.Authors.Add(a.PersonId) },
        VersionFrozen v => m with { Frozen = [.. v.Sections], LatestVersion = v.Number },
        ReviewRoundOpened o => m with { Status = Status.UnderReview, Round = o.Round },
        DecisionMade { Decision: Decision.Accept } => m with { Status = Status.Accepted },
        DecisionMade { Decision: Decision.Reject } => m with { Status = Status.Rejected },
        DecisionMade => m with { Status = Status.InRevision },
        Published => m with { Status = Status.Published },
        UpdateIssued { Type: UpdateType.Retraction } => m with { Status = Status.Retracted },
        _ => m,
    };

    public static ImmutableList<SectionRef> Put(ImmutableList<SectionRef> sections, SectionRef section) =>
        sections.FindIndex(s => s.SectionId == section.SectionId) is var i and >= 0 ? sections.SetItem(i, section) : sections.Add(section);
}
```
<!-- endSnippet -->


3. Write the decisions.

   <!-- snippet: publishing-decider -->
```cs
public static class Editorial
{
    public static IEnumerable<object> Start(Manuscript m, string title)
    {
        if (m.Title is not null)
            throw new InvalidOperationException("The manuscript exists already.");
        yield return new ManuscriptStarted(title);
    }

    public static IEnumerable<object> AddAuthor(Manuscript m, string personId, string name, string? orcid, string affiliation, bool corresponding = false)
    {
        if (m.Status is not (Status.Draft or Status.InRevision))
            throw new InvalidOperationException($"Authors cannot change while the manuscript is {m.Status}.");
        if (m.Authors.Contains(personId))
            throw new InvalidOperationException($"{personId} is an author already.");
        yield return new AuthorAdded(personId, name, orcid, affiliation, corresponding);
    }

    // Authors edit the working copy before submission and during a revision, never while reviewers read it.
    public static IEnumerable<object> Revise(Manuscript m, string sectionId, string heading, string contentHash)
    {
        if (m.Status is not (Status.Draft or Status.InRevision))
            throw new InvalidOperationException($"Sections cannot change while the manuscript is {m.Status}.");
        yield return new SectionRevised(sectionId, heading, contentHash);
    }

    // Submitting freezes the working copy, and pins the new review round to exactly that version.
    public static IEnumerable<object> Submit(Manuscript m)
    {
        if (m.Status is not (Status.Draft or Status.InRevision) || m.Draft.IsEmpty)
            throw new InvalidOperationException($"A {m.Status} manuscript with {m.Draft.Count} sections cannot be submitted.");
        var version = Freeze(m, Stage.SubmittedUnderReview, m.Draft, m.Round == 0 ? null : $"Response to round {m.Round}");
        yield return version;
        yield return new ReviewRoundOpened(m.Round + 1, version.Number);
    }

    // Acceptance freezes the reviewed content again, as the Accepted Manuscript.
    public static IEnumerable<object> Decide(Manuscript m, Decision decision)
    {
        if (m.Status is not Status.UnderReview)
            throw new InvalidOperationException("Only a manuscript under review gets a decision.");
        yield return new DecisionMade(m.Round, decision);
        if (decision is Decision.Accept)
            yield return Freeze(m, Stage.AcceptedManuscript, m.Frozen, null);
    }

    // An editor fixes an accepted manuscript without a new review round: a new version at the same stage, with a reason.
    public static IEnumerable<object> EditorialChange(Manuscript m, string sectionId, string heading, string contentHash, string reason)
    {
        if (m.Status is not Status.Accepted)
            throw new InvalidOperationException("Changes without review apply to accepted manuscripts only.");
        yield return new SectionRevised(sectionId, heading, contentHash);
        yield return Freeze(m, Stage.AcceptedManuscript, Manuscript.Put(m.Frozen, new(sectionId, heading, contentHash)), reason);
    }

    public static IEnumerable<object> Publish(Manuscript m, string doi)
    {
        if (m.Status is not Status.Accepted)
            throw new InvalidOperationException("Only an accepted manuscript is published.");
        var record = Freeze(m, Stage.VersionOfRecord, m.Frozen, null);
        yield return record;
        yield return new Published(record.Number, doi);
    }

    // Crossref: the article keeps its DOI, the notice gets its own, and the published version never changes.
    public static IEnumerable<object> Correct(Manuscript m, string sectionId, string heading, string contentHash, string noticeDoi)
    {
        if (m.Status is not Status.Published)
            throw new InvalidOperationException("Only a published article is corrected.");
        var corrected = Freeze(m, Stage.CorrectedVersionOfRecord, Manuscript.Put(m.Frozen, new(sectionId, heading, contentHash)), "Correction");
        yield return new SectionRevised(sectionId, heading, contentHash);
        yield return corrected;
        yield return new UpdateIssued(UpdateType.Correction, noticeDoi, corrected.Number);
    }

    // NISO CREC: a retracted article stays readable and marked. No version is deleted.
    public static IEnumerable<object> Retract(Manuscript m, string noticeDoi)
    {
        if (m.Status is not Status.Published)
            throw new InvalidOperationException("Only a published article is retracted.");
        yield return new UpdateIssued(UpdateType.Retraction, noticeDoi, null);
    }

    private static VersionFrozen Freeze(Manuscript m, Stage stage, IReadOnlyList<SectionRef> sections, string? reason) =>
        new(m.LatestVersion + 1, stage, sections, m.LatestVersion == 0 ? null : m.LatestVersion, reason);
}
```
<!-- endSnippet -->


   | Rule | Where |
   | --- | --- |
   | Authors edit the working copy only in a draft or during a revision. | `Revise` |
   | Submitting freezes the working copy and pins the review round to that version. | `Submit` |
   | Acceptance freezes the reviewed content again, as the Accepted Manuscript. | `Decide` |
   | An editor changes an accepted manuscript without a new round: a new version at the same stage, with a reason. | `EditorialChange` |
   | A correction adds a Corrected Version of Record and a notice. The earlier versions stay. | `Correct` |

4. Test the decisions. They are pure functions, so the tests need no database.

   <!-- snippet: publishing-decider-tests -->
```cs
public class EditorialTests
{
    private static readonly SectionRef Intro = new("intro", "Introduction", "h1");

    [Fact]
    public void Submitting_freezes_the_draft_and_pins_the_round_to_that_version() =>
        Decider.Given<Manuscript>(new ManuscriptStarted("T"), new SectionRevised("intro", "Introduction", "h1"))
            .When(Editorial.Submit)
            .Then(new VersionFrozen(1, Stage.SubmittedUnderReview, [Intro], null, null), new ReviewRoundOpened(1, 1));

    [Fact]
    public void Authors_cannot_change_a_version_that_reviewers_read() =>
        Decider.Given<Manuscript>(new ManuscriptStarted("T"), new SectionRevised("intro", "Introduction", "h1"),
                new VersionFrozen(1, Stage.SubmittedUnderReview, [Intro], null, null), new ReviewRoundOpened(1, 1))
            .When(m => Editorial.Revise(m, "intro", "Introduction", "h2"))
            .ThenThrows<InvalidOperationException>();

    [Fact]
    public void Acceptance_freezes_the_reviewed_content_as_the_accepted_manuscript() =>
        Decider.Given<Manuscript>(new ManuscriptStarted("T"), new SectionRevised("intro", "Introduction", "h1"),
                new VersionFrozen(1, Stage.SubmittedUnderReview, [Intro], null, null), new ReviewRoundOpened(1, 1))
            .When(m => Editorial.Decide(m, Decision.Accept))
            .Then(new DecisionMade(1, Decision.Accept), new VersionFrozen(2, Stage.AcceptedManuscript, [Intro], 1, null));
}
```
<!-- endSnippet -->


5. Write the views, which are the shapes the API returns, and the read model, which holds them.

   <!-- snippet: publishing-views -->
```cs
// The shapes the API returns. ManuscriptView is also stored whole, as one JSON document per manuscript.
public record ManuscriptView(
    string Id, string Title, Status Status, string? Doi,
    IReadOnlyList<AuthorView> Authors,
    VersionView? Latest,      // the newest version, for authors and editors
    VersionView? Published,   // the version readers see: the Version of Record, or its latest correction
    IReadOnlyList<RoundView> Rounds,
    IReadOnlyList<UpdateView> Updates);

public record ManuscriptSummary(string Id, string Title, Status Status, int? LatestVersion, int? PublishedVersion, string? Doi, DateTimeOffset UpdatedAt);

public record ManuscriptPage(IReadOnlyList<ManuscriptSummary> Items, string? Next);

public record VersionView(int Number, Stage Stage, int? BasedOn, string? Reason, DateTimeOffset FrozenAt, IReadOnlyList<SectionView> Sections);

public record VersionSummary(string ManuscriptId, int Number, Stage Stage, int? BasedOn, string? Reason, DateTimeOffset FrozenAt);

public record SectionView(string SectionId, string Heading, string ContentUrl);

public record SectionChange(string SectionId, string Heading, string Change);  // added, changed or removed

public record AuthorView(string PersonId, string? Name, string Affiliation, bool Corresponding);

public record PersonSummary(string PersonId, string? Name, string? Orcid, int Manuscripts);

public record PersonView(string PersonId, string? Name, string? Orcid, IReadOnlyList<Authorship> Manuscripts);

public record Authorship(string ManuscriptId, string Title, Status Status, string Affiliation, bool Corresponding);

public record InstitutionCount(string Affiliation, int People, int Manuscripts);

public record RoundView(int Round, int Version, Decision? Decision);

public record UpdateView(UpdateType Type, string NoticeDoi, int? Version, DateTimeOffset IssuedAt);
```
<!-- endSnippet -->


   <!-- snippet: publishing-read-model -->
```cs
// One row per manuscript. The columns serve the list; the document serves the detail page in one read.
public class ManuscriptRow
{
    public required string Id { get; set; }
    public required string Title { get; set; }
    public Status Status { get; set; }
    public int? LatestVersion { get; set; }
    public int? PublishedVersion { get; set; }
    public string? Doi { get; set; }
    public DateTimeOffset UpdatedAt { get; set; }
    public required string Document { get; set; }  // ManuscriptView as jsonb
}

// One row per frozen version, and one per section of it: the version history.
public class VersionRow
{
    public required string ManuscriptId { get; set; }
    public int Number { get; set; }
    public Stage Stage { get; set; }
    public int? BasedOn { get; set; }
    public string? Reason { get; set; }
    public DateTimeOffset FrozenAt { get; set; }
}

public class VersionSectionRow
{
    public required string ManuscriptId { get; set; }
    public int Version { get; set; }
    public int Position { get; set; }
    public required string SectionId { get; set; }
    public required string Heading { get; set; }
    public required string ContentHash { get; set; }
}

// One row per person, and one per authorship: who wrote what, across every manuscript.
public class PersonRow
{
    public required string PersonId { get; set; }
    public string? Name { get; set; }   // null once the person is erased
    public string? Orcid { get; set; }
}

public class ManuscriptAuthorRow
{
    public required string ManuscriptId { get; set; }
    public required string PersonId { get; set; }
    public long Position { get; set; }  // byline order
    public required string Affiliation { get; set; }
    public bool Corresponding { get; set; }
}

public class PublishingDb(DbContextOptions<PublishingDb> options) : DbContext(options)
{
    public DbSet<PersonRow> People => Set<PersonRow>();
    public DbSet<ManuscriptAuthorRow> ManuscriptAuthors => Set<ManuscriptAuthorRow>();
    public DbSet<ManuscriptRow> Manuscripts => Set<ManuscriptRow>();
    public DbSet<VersionRow> Versions => Set<VersionRow>();
    public DbSet<VersionSectionRow> VersionSections => Set<VersionSectionRow>();

    protected override void ConfigureConventions(ModelConfigurationBuilder conventions) =>
        conventions.Properties<Enum>().HaveConversion<string>();  // store enums as their names

    protected override void OnModelCreating(ModelBuilder model)
    {
        model.HasDefaultSchema("publishing");
        model.HasPostgresExtension("pg_trgm");
        var manuscripts = model.Entity<ManuscriptRow>().ToTable("manuscripts");
        manuscripts.HasKey(m => m.Id);
        manuscripts.Property(m => m.Document).HasColumnType("jsonb");
        manuscripts.HasIndex(m => new { m.UpdatedAt, m.Id });                                 // keyset paging
        manuscripts.HasIndex(m => m.Title).HasMethod("gin").HasOperators("gin_trgm_ops");  // title search
        model.Entity<VersionRow>().ToTable("versions").HasKey(v => new { v.ManuscriptId, v.Number });
        model.Entity<VersionSectionRow>().ToTable("version_sections").HasKey(s => new { s.ManuscriptId, s.Version, s.Position });

        var people = model.Entity<PersonRow>().ToTable("people");
        people.HasKey(p => p.PersonId);
        people.HasIndex(p => p.Name).HasMethod("gin").HasOperators("gin_trgm_ops");               // search by name
        var authors = model.Entity<ManuscriptAuthorRow>().ToTable("manuscript_authors");
        authors.HasKey(a => new { a.ManuscriptId, a.PersonId });
        authors.HasIndex(a => a.PersonId);                                                          // manuscripts by person
        authors.HasIndex(a => a.Affiliation).HasMethod("gin").HasOperators("gin_trgm_ops");       // search by affiliation
    }
}

public static class Documents
{
    private static readonly JsonSerializerOptions Json = new(JsonSerializerDefaults.Web) { Converters = { new JsonStringEnumConverter() } };

    public static ManuscriptView Read(string json) => JsonSerializer.Deserialize<ManuscriptView>(json, Json)!;

    public static string Write(ManuscriptView view) => JsonSerializer.Serialize(view, Json);
}
```
<!-- endSnippet -->


   | Table | Holds | Serves |
   | --- | --- | --- |
   | `manuscripts` | The title, status, DOI and version numbers as columns, and the whole `ManuscriptView` as a `jsonb` document | The list, from the columns; the detail page, from the document |
   | `versions` and `version_sections` | Every frozen version and its sections | The history, one version, and changes between two |
   | `people` and `manuscript_authors` | One profile per person, and one row per authorship | Manuscripts by person, search by name or affiliation, counts per person and per institution |

   The document is a read model built for one query: the detail page reads one row by its key. Writes are rare, so the projection can rewrite the document on every event. The history stays in its own tables, because a manuscript can have many versions.

6. Write the projection. It runs inline, in the append's transaction, so a read right after a command sees the command's effect.

   <!-- snippet: publishing-projection -->
```cs
// Inline, so an API call right after a command reads its own write.
public sealed class ManuscriptProjection : Projection<PublishingDb>
{
    public ManuscriptProjection()
    {
        On<ManuscriptStarted>((e, ctx) =>
        {
            var document = new ManuscriptView(ctx.StreamId, e.Title, Status.Draft, null, [], null, null, [], []);
            ctx.Db.Manuscripts.Add(new ManuscriptRow { Id = ctx.StreamId, Title = e.Title, UpdatedAt = ctx.OccurredAt, Document = Documents.Write(document) });
            return Task.CompletedTask;
        });

        On<AuthorAdded>((e, ctx) => Change(ctx, m => m with { Authors = [.. m.Authors, new AuthorView(e.PersonId, e.Name, e.Affiliation, e.Corresponding)] }));

        On<VersionFrozen>(async (e, ctx) =>
        {
            var sections = e.Sections.Select(s => new SectionView(s.SectionId, s.Heading, $"/content/{s.ContentHash}")).ToList();
            await Change(ctx, m => m with { Latest = new VersionView(e.Number, e.Stage, e.BasedOn, e.Reason, ctx.OccurredAt, sections) });

            ctx.Db.Versions.Add(new VersionRow
            {
                ManuscriptId = ctx.StreamId, Number = e.Number, Stage = e.Stage, BasedOn = e.BasedOn, Reason = e.Reason, FrozenAt = ctx.OccurredAt,
            });
            ctx.Db.VersionSections.AddRange(e.Sections.Select((s, i) => new VersionSectionRow
            {
                ManuscriptId = ctx.StreamId, Version = e.Number, Position = i, SectionId = s.SectionId, Heading = s.Heading, ContentHash = s.ContentHash,
            }));
        });

        On<ReviewRoundOpened>((e, ctx) => Change(ctx, m => m with
        {
            Status = Status.UnderReview,
            Rounds = [.. m.Rounds, new RoundView(e.Round, e.Version, null)],
        }));

        On<DecisionMade>((e, ctx) => Change(ctx, m => m with
        {
            Status = e.Decision switch { Decision.Accept => Status.Accepted, Decision.Reject => Status.Rejected, _ => Status.InRevision },
            Rounds = [.. m.Rounds.Select(r => r.Round == e.Round ? r with { Decision = e.Decision } : r)],
        }));

        On<Published>((e, ctx) => Change(ctx, m => m with { Status = Status.Published, Doi = e.Doi, Published = m.Latest }));

        On<UpdateIssued>((e, ctx) => Change(ctx, m => m with
        {
            Status = e.Type is UpdateType.Retraction ? Status.Retracted : m.Status,
            Published = e.Type is UpdateType.Correction ? m.Latest : m.Published,  // the corrected version is the newest one
            Updates = [.. m.Updates, new UpdateView(e.Type, e.NoticeDoi, e.Version, ctx.OccurredAt)],
        }));

        // Erasure deletes the author's key; the read model drops the name too.
        On<SubjectErased>((e, ctx) => Change(ctx, m => m with
        {
            Authors = [.. m.Authors.Select(a => a.PersonId == e.SubjectId ? a with { Name = null } : a)],
        }));
    }

    // A rebuild calls this, then replays every event.
    protected override async Task ResetAsync(WriteContext<PublishingDb> context)
    {
        var (db, ct) = (context.Db, context.CancellationToken);
        await db.VersionSections.ExecuteDeleteAsync(ct);
        await db.Versions.ExecuteDeleteAsync(ct);
        await db.Manuscripts.ExecuteDeleteAsync(ct);
    }

    // Changes the document, then copies the fields that the list filters and sorts on into their columns.
    private static async Task Change(ProjectionContext<PublishingDb> ctx, Func<ManuscriptView, ManuscriptView> change)
    {
        var row = (await ctx.Db.Manuscripts.FindAsync([ctx.StreamId], ctx.CancellationToken))!;
        var document = change(Documents.Read(row.Document));
        row.Document = Documents.Write(document);
        (row.Status, row.Doi, row.LatestVersion, row.PublishedVersion, row.UpdatedAt) =
            (document.Status, document.Doi, document.Latest?.Number, document.Published?.Number, ctx.OccurredAt);
    }
}
```
<!-- endSnippet -->


   Inline suits editorial screens: an author who submits sees the new version at once, and manuscripts get few writes. Register a second projection with `Run.Async` for work that may lag, such as a search index or the public site. See [inline or async projections](/concepts/inline-or-async/).

   Each handler changes the document, and `Change` copies the fields that the list filters and sorts on into their columns. The projection also handles `SubjectErased`: when an author is erased, Deedbox deletes their key and appends `SubjectErased`, and the projection clears their name.

7. Write a second projection for people. Questions across manuscripts, such as "every manuscript by this person", need rows to filter and join on, not a document per manuscript.

   <!-- snippet: publishing-people-projection -->
```cs
// A second read model from the same events, for questions across manuscripts. It rebuilds on its own.
public sealed class PeopleProjection : Projection<PublishingDb>
{
    public PeopleProjection()
    {
        On<AuthorAdded>(async (e, ctx) =>
        {
            // One profile per person: the newest name and ORCID win.
            if (await ctx.Db.People.FindAsync([e.PersonId], ctx.CancellationToken) is { } person)
                (person.Name, person.Orcid) = (e.Name, e.Orcid);
            else
                ctx.Db.People.Add(new PersonRow { PersonId = e.PersonId, Name = e.Name, Orcid = e.Orcid });

            ctx.Db.ManuscriptAuthors.Add(new ManuscriptAuthorRow
            {
                ManuscriptId = ctx.StreamId, PersonId = e.PersonId, Position = ctx.Version, Affiliation = e.Affiliation, Corresponding = e.Corresponding,
            });
        });

        // Erasure reaches every manuscript of the person; each one clears the same profile.
        On<SubjectErased>(async (e, ctx) =>
        {
            if (await ctx.Db.People.FindAsync([e.SubjectId], ctx.CancellationToken) is { } person)
                (person.Name, person.Orcid) = (null, null);
        });
    }

    protected override async Task ResetAsync(WriteContext<PublishingDb> context)
    {
        await context.Db.ManuscriptAuthors.ExecuteDeleteAsync(context.CancellationToken);
        await context.Db.People.ExecuteDeleteAsync(context.CancellationToken);
    }
}
```
<!-- endSnippet -->


   Both projections read the same events, and each one keeps its own checkpoint, so you can rebuild one without the other. The detail page still shows the authors from the manuscript document, so it stays one read.

8. Write the queries. Every read goes through this class.

   <!-- snippet: publishing-queries -->
```cs
// Every read goes through here. Each method opens its own context, so parallel callers such as GraphQL resolvers are safe.
public sealed class ManuscriptQueries(IDbContextFactory<PublishingDb> contexts)
{
    // The list: newest change first, with keyset paging and a case-insensitive title search.
    public async Task<ManuscriptPage> List(string? search, Status? status, string? after, int limit, CancellationToken ct = default)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        var query = db.Manuscripts.AsNoTracking();
        if (!string.IsNullOrWhiteSpace(search))
            query = query.Where(m => EF.Functions.ILike(m.Title, $"%{Escape(search)}%", @"\"));
        if (status is not null)
            query = query.Where(m => m.Status == status);
        if (Cursor.Read(after) is var (updatedAt, id))
            query = query.Where(m => m.UpdatedAt < updatedAt || (m.UpdatedAt == updatedAt && string.Compare(m.Id, id) > 0));

        var rows = await query.OrderByDescending(m => m.UpdatedAt).ThenBy(m => m.Id).Take(limit + 1)
            .Select(m => new ManuscriptSummary(m.Id, m.Title, m.Status, m.LatestVersion, m.PublishedVersion, m.Doi, m.UpdatedAt))
            .ToListAsync(ct);
        var next = rows.Count > limit ? Cursor.Write(rows[limit - 1].UpdatedAt, rows[limit - 1].Id) : null;
        return new ManuscriptPage(rows.Take(limit).ToList(), next);
    }

    // The detail page: one primary-key read of the stored document.
    public async Task<ManuscriptView?> Manuscript(string id, CancellationToken ct = default)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        var document = await db.Manuscripts.Where(m => m.Id == id).Select(m => m.Document).SingleOrDefaultAsync(ct);
        return document is null ? null : Documents.Read(document);
    }

    public async Task<List<VersionSummary>> Versions(string id, CancellationToken ct = default)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        return await db.Versions.AsNoTracking().Where(v => v.ManuscriptId == id).OrderBy(v => v.Number)
            .Select(v => new VersionSummary(v.ManuscriptId, v.Number, v.Stage, v.BasedOn, v.Reason, v.FrozenAt)).ToListAsync(ct);
    }

    // One version with its sections, in one query.
    public async Task<VersionView?> Version(string id, int number, CancellationToken ct = default)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        return await db.Versions.AsNoTracking().Where(v => v.ManuscriptId == id && v.Number == number)
            .Select(v => new VersionView(v.Number, v.Stage, v.BasedOn, v.Reason, v.FrozenAt,
                db.VersionSections.Where(s => s.ManuscriptId == id && s.Version == number).OrderBy(s => s.Position)
                    .Select(s => new SectionView(s.SectionId, s.Heading, "/content/" + s.ContentHash)).ToList()))
            .SingleOrDefaultAsync(ct);
    }

    // What changed between two versions, section by section, from the content hashes. One query reads both versions.
    public async Task<List<SectionChange>> Changes(string id, int from, int to, CancellationToken ct = default)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        var sections = await db.VersionSections.AsNoTracking()
            .Where(s => s.ManuscriptId == id && (s.Version == from || s.Version == to)).OrderBy(s => s.Position).ToListAsync(ct);
        var before = sections.Where(s => s.Version == from).ToDictionary(s => s.SectionId);
        var changes = new List<SectionChange>();
        foreach (var s in sections.Where(s => s.Version == to))
        {
            if (!before.Remove(s.SectionId, out var old))
                changes.Add(new(s.SectionId, s.Heading, "added"));
            else if (old.ContentHash != s.ContentHash || old.Heading != s.Heading)
                changes.Add(new(s.SectionId, s.Heading, "changed"));
        }

        changes.AddRange(before.Values.Select(s => new SectionChange(s.SectionId, s.Heading, "removed")));
        return changes;
    }

    // People, most prolific first. The search matches a name or any affiliation the person wrote under.
    public async Task<List<PersonSummary>> People(string? search, int limit, CancellationToken ct = default)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        var people = db.People.AsNoTracking();
        if (!string.IsNullOrWhiteSpace(search))
        {
            var pattern = $"%{Escape(search)}%";
            people = people.Where(p => EF.Functions.ILike(p.Name!, pattern, @"")
                || db.ManuscriptAuthors.Any(a => a.PersonId == p.PersonId && EF.Functions.ILike(a.Affiliation, pattern, @"")));
        }

        return await people
            .Select(p => new { p.PersonId, p.Name, p.Orcid, Manuscripts = db.ManuscriptAuthors.Count(a => a.PersonId == p.PersonId) })
            .OrderByDescending(p => p.Manuscripts).ThenBy(p => p.PersonId).Take(limit)
            .Select(p => new PersonSummary(p.PersonId, p.Name, p.Orcid, p.Manuscripts)).ToListAsync(ct);
    }

    // One profile with every manuscript the person wrote, in one query.
    public async Task<PersonView?> Person(string personId, CancellationToken ct = default)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        return await db.People.AsNoTracking().Where(p => p.PersonId == personId)
            .Select(p => new PersonView(p.PersonId, p.Name, p.Orcid,
                db.ManuscriptAuthors.Where(a => a.PersonId == personId)
                    .Join(db.Manuscripts, a => a.ManuscriptId, m => m.Id, (a, m) => new { a, m })
                    .OrderByDescending(x => x.m.UpdatedAt)
                    .Select(x => new Authorship(x.m.Id, x.m.Title, x.m.Status, x.a.Affiliation, x.a.Corresponding)).ToList()))
            .SingleOrDefaultAsync(ct);
    }

    // Authors and manuscripts per affiliation, most manuscripts first.
    public async Task<List<InstitutionCount>> Institutions(string? search, int limit, CancellationToken ct = default)
    {
        await using var db = await contexts.CreateDbContextAsync(ct);
        var authors = db.ManuscriptAuthors.AsNoTracking();
        if (!string.IsNullOrWhiteSpace(search))
            authors = authors.Where(a => EF.Functions.ILike(a.Affiliation, $"%{Escape(search)}%", @""));
        return await authors.GroupBy(a => a.Affiliation)
            .Select(g => new { Affiliation = g.Key, People = g.Select(a => a.PersonId).Distinct().Count(), Manuscripts = g.Select(a => a.ManuscriptId).Distinct().Count() })
            .OrderByDescending(i => i.Manuscripts).ThenBy(i => i.Affiliation).Take(limit)
            .Select(i => new InstitutionCount(i.Affiliation, i.People, i.Manuscripts)).ToListAsync(ct);
    }

    private static string Escape(string text) => text.Replace(@"\", @"\\").Replace("%", @"\%").Replace("_", @"\_");
}

// An opaque cursor: the sort values of the last row on the page.
public static class Cursor
{
    public static string Write(DateTimeOffset updatedAt, string id) =>
        Convert.ToBase64String(Encoding.UTF8.GetBytes($"{updatedAt.UtcTicks}:{id}"));

    public static (DateTimeOffset UpdatedAt, string Id)? Read(string? cursor)
    {
        if (string.IsNullOrEmpty(cursor))
            return null;
        var text = Encoding.UTF8.GetString(Convert.FromBase64String(cursor));
        var colon = text.IndexOf(':');
        return (new DateTimeOffset(long.Parse(text[..colon]), TimeSpan.Zero), text[(colon + 1)..]);
    }
}
```
<!-- endSnippet -->


   | Query | Database work |
   | --- | --- |
   | `List` | One query on the `manuscripts` columns. Keyset paging stays correct while manuscripts change, unlike page numbers. The title search is case-insensitive, and a trigram index keeps it fast. |
   | `Manuscript` | One read of the document by its key |
   | `Versions`, `Version`, `Changes` | One query each. `Changes` compares the content hashes of two versions, so no event stores a diff. |
   | `People` | One query. It matches a name, or any affiliation the person wrote under, and sorts by manuscript count. |
   | `Person` | One query: the profile and every manuscript, with the affiliation of each authorship |
   | `Institutions` | One query that counts people and manuscripts per affiliation |

9. Serve the queries over REST, and send commands through the stream.

   <!-- snippet: publishing-rest -->
```cs
public static class ManuscriptEndpoints
{
    public static void MapManuscripts(this IEndpointRouteBuilder app)
    {
        var api = app.MapGroup("/manuscripts");

        // Reads come from the projection.
        api.MapGet("/", async (string? search, Status? status, string? after, int? limit, ManuscriptQueries q, CancellationToken ct) =>
            Results.Ok(await q.List(search, status, after, Math.Clamp(limit ?? 20, 1, 100), ct)));
        api.MapGet("/{id}", async (string id, ManuscriptQueries q, CancellationToken ct) =>
            await q.Manuscript(id, ct) is { } m ? Results.Ok(m) : Results.NotFound());
        api.MapGet("/{id}/versions", async (string id, ManuscriptQueries q, CancellationToken ct) =>
            Results.Ok(await q.Versions(id, ct)));
        api.MapGet("/{id}/versions/{number:int}", async (string id, int number, ManuscriptQueries q, CancellationToken ct) =>
            await q.Version(id, number, ct) is { } v ? Results.Ok(v) : Results.NotFound());
        api.MapGet("/{id}/versions/{number:int}/changes", async (string id, int number, int from, ManuscriptQueries q, CancellationToken ct) =>
            Results.Ok(await q.Changes(id, from, number, ct)));

        // People and institutions come from the second read model.
        app.MapGet("/people", async (string? search, int? limit, ManuscriptQueries q, CancellationToken ct) =>
            Results.Ok(await q.People(search, Math.Clamp(limit ?? 20, 1, 100), ct)));
        app.MapGet("/people/{personId}", async (string personId, ManuscriptQueries q, CancellationToken ct) =>
            await q.Person(personId, ct) is { } p ? Results.Ok(p) : Results.NotFound());
        app.MapGet("/institutions", async (string? search, int? limit, ManuscriptQueries q, CancellationToken ct) =>
            Results.Ok(await q.Institutions(search, Math.Clamp(limit ?? 20, 1, 100), ct)));

        // Commands go through the stream. A rule the decider enforces becomes 409 Conflict.
        api.MapPost("/{id}/submit", (string id, IEventStore store, CancellationToken ct) =>
            Run(() => store.Execute<Manuscript>(id, Editorial.Submit, ct)));
        api.MapPost("/{id}/decision", (string id, Decision decision, IEventStore store, CancellationToken ct) =>
            Run(() => store.Execute<Manuscript>(id, m => Editorial.Decide(m, decision), ct)));
    }

    private static async Task<IResult> Run(Func<Task<ExecuteResult<Manuscript>>> command)
    {
        try
        {
            var result = await command();
            return Results.Ok(new { result.Version });
        }
        catch (InvalidOperationException ex)
        {
            return Results.Conflict(new { error = ex.Message });
        }
    }
}
```
<!-- endSnippet -->


10. Register everything in `Program.cs`, and map the endpoints.

    <Register />

    <Map />

    Create the read model tables with an EF Core migration, or with `EnsureCreated()` while you try this out. The model adds the `pg_trgm` extension for the title index.

</docs.Steps>

## Try it

```sh
curl "localhost:5000/manuscripts?search=frozen&limit=20"                     # the list; follow "next" for the next page
curl -X POST localhost:5000/manuscripts/m-1/submit                           # version 1, round 1
curl -X POST "localhost:5000/manuscripts/m-1/decision?decision=MajorRevision"
curl localhost:5000/manuscripts/m-1                                          # latest and published versions, in one read
curl localhost:5000/manuscripts/m-1/versions                                 # the history
curl localhost:5000/manuscripts/m-1/versions/2                               # one frozen version
curl "localhost:5000/manuscripts/m-1/versions/2/changes?from=1"              # what the revision changed
curl "localhost:5000/people?search=royal"                                    # people by name or affiliation
curl localhost:5000/people/person:ada                                        # one profile and every manuscript
curl localhost:5000/institutions                                             # people and manuscripts per affiliation
```

A command that breaks a rule, such as submitting a published article, returns 409 Conflict with the decider's message.

To serve the same read model over GraphQL, see [serve a read model over GraphQL](/how-to/serve-graphql/).

## Next steps

- Move review rounds to their own stream, such as `review-m-1-2`, with reviewer invitations and reports as `[PersonalData]` events. A [subscription](/concepts/projections-and-subscriptions/) links the round decision to the manuscript. Do not append to two streams in one transaction.
- Start an `article-{doi}` stream at publication, so post-publication events stay apart from the editorial history.
- Send `Published` and `UpdateIssued` to your Crossref deposit service with [QueueBox](/how-to/wire-queuebox/), as CloudEvents if it expects them.
- Add the other stages when you need them: preprints, proofs, and the Enhanced Version of Record.
- Rebuild a read model after you change its logic. See [rebuild a projection](/how-to/rebuild-a-projection/). To change its tables while the old ones serve reads, see [replace a read model without downtime](/how-to/replace-a-read-model/).
