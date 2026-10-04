# Collapsible

One panel that opens on demand.

## Usage

```gx
<collapsible.Collapsible summary={gx.Text("Show details")}>
  <p>Hidden content.</p>
</collapsible.Collapsible>
```

The panel uses the native `<details>` element.

## Do

- Use it for one optional block of content.
- Put a verb in the summary.

## Don't

- Do not hide essential content.
- Do not use a collapsible for a list of questions. Use an accordion.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the summary. |
| Enter, Space | Opens or closes the panel. |
