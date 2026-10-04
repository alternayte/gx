# App Shell

An app page frame with a sidebar, a header and a main area.

## Usage

```gx
<appshell.AppShell title="Acme" nav={<appshell.Nav />}>
  <p>Page content.</p>
</appshell.AppShell>
```

The block is copied source. Wrap it in a `gx.Layout` so every page shares the frame.

## Do

- Keep one page title in the header.
- Put the primary navigation in `Nav`.

## Don't

- Do not add a second header inside the main area.
- Do not nest an app shell in another app shell.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves to the menu button, then the sidebar links, then the page. |
| Enter, Space | Opens or closes the sidebar on a narrow screen. |
