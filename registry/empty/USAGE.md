# Empty

A placeholder for a view with no data.

## Usage

```gx
<empty.Empty>
  <empty.EmptyHeader>
    <empty.EmptyMedia variant={empty.Icon}><icons.Info /></empty.EmptyMedia>
    <empty.EmptyTitle>No projects</empty.EmptyTitle>
    <empty.EmptyDescription>Create your first project to start.</empty.EmptyDescription>
  </empty.EmptyHeader>
  <empty.EmptyContent>
    <button.Button>New project</button.Button>
  </empty.EmptyContent>
</empty.Empty>
```

The empty state has no visible border. Add `class="border"` for a dashed outline.
`EmptyMedia` with the `Icon` variant draws a tile behind the icon.

## Do

- Say what is missing in `EmptyTitle`.
- Put the action that fixes the state in `EmptyContent`.

## Don't

- Do not use an empty state for an error. Use an alert.
- Do not put more than two actions in `EmptyContent`.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
