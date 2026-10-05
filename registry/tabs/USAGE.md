# Tabs

Exclusive panels behind a list of tab buttons.

## Usage

```gx
<tabs.Tabs default="Account">
  <tabs.TabsList label="Settings">
    <tabs.TabsTrigger label="Account" />
    <tabs.TabsTrigger label="Password" />
  </tabs.TabsList>
  <tabs.TabsContent label="Account">Account settings.</tabs.TabsContent>
  <tabs.TabsContent label="Password">Password settings.</tabs.TabsContent>
</tabs.Tabs>
```

A trigger and its panel share one `Label`.
`TabsList` has the variants `tabs.Default` (a muted pill) and `tabs.Line` (an underline).
Set `Orientation` to `tabs.Vertical` to stack the list beside the panels.
Two tab groups with the same `Sync` key keep one selection and remember it per viewer.
The behaviour runtime selects the tab. Without JavaScript, every panel shows.

## Do

- Put a short label on each tab.
- Give the list a `Label` that names the group.
- Use `Sync` when two groups must move together.

## Don't

- Do not hide primary navigation in tabs.
- Do not use tabs for steps of a process. Use a page or a form.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Enters the tab list at the selected tab. Then moves to the panel. |
| Arrow Right, Arrow Left | Selects the next or previous tab of a horizontal list. |
| Arrow Down, Arrow Up | Selects the next or previous tab of a vertical list. |
| Home, End | Selects the first or last tab. |
| Enter, Space | Selects the focused tab. |
