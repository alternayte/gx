# Empty

A placeholder for an empty result.

## Usage

```gx
<empty.Empty>
  <empty.EmptyHeader>
    <empty.EmptyTitle>No projects</empty.EmptyTitle>
    <empty.EmptyDescription>Create your first project to start.</empty.EmptyDescription>
  </empty.EmptyHeader>
  <empty.EmptyContent>
    <a href={projects.New{}}>New project</a>
  </empty.EmptyContent>
</empty.Empty>
```

## Do

- Explain why the area is empty.
- Offer the next action in `EmptyContent`.

## Don't

- Do not use an empty state for an error. Use an alert.
- Do not hide the primary action of the page.

## Keyboard

This component is static. It takes no focus and has no key bindings.
