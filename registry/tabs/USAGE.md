# Tabs

Exclusive panels behind a row of tab buttons.

## Usage

```gx
<tabs.Tabs default="Account">
  <tabs.TabItem label="Account">Account settings.</tabs.TabItem>
  <tabs.TabItem label="Password">Password settings.</tabs.TabItem>
</tabs.Tabs>
```

Two tab groups with the same `Sync` key keep one selection and remember it per viewer.

## Do

- Put a short label on each tab.
- Use `Sync` when two groups must move together.

## Don't

- Do not hide primary navigation in tabs.
- Do not use tabs for steps of a process. Use a page or a form.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Enters the tab list at the selected tab. |
| Enter, Space | Selects the focused tab. |
