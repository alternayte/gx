# Progress

A bar that shows the completion of a task.

## Usage

```gx
<progress.Progress value={p.Done} max={p.Total} label="Upload" />
```

## Do

- Set `Label` so the bar has an accessible name.
- Update the value as the task advances.

## Don't

- Do not use a progress bar for an unknown duration. Use a spinner.
- Do not animate the bar backward without a reason.

## Keyboard

This component is static. It takes no focus and has no key bindings.
