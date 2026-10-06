# Resizable

A group of panels with a handle that changes their sizes.

## Usage

```gx
<resizable.ResizablePanelGroup class="h-64 rounded-md border">
  <resizable.ResizablePanel size={30} minSize={20}>Sidebar</resizable.ResizablePanel>
  <resizable.ResizableHandle id="sidebar-handle" />
  <resizable.ResizablePanel size={70}>Content</resizable.ResizablePanel>
</resizable.ResizablePanelGroup>
```

The panels are server HTML, and `Size` is the first size of a panel in percent of the group.
The island of a handle changes the sizes of the panel before it and the panel after it.
The handle sends the `resize-panels` event with the two sizes.
With no script the page shows the panels with their first sizes.

## Do

- Put one `ResizableHandle` between two panels.
- Give the group a height. A vertical group needs one.
- Set `MinSize` for a panel whose content needs room.

## Don't

- Do not put a handle at the start or the end of a group.
- Do not use two handles with one `Id`.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the handle. |
| Arrow Left, Arrow Right | Makes the first panel smaller or larger. A vertical group uses Arrow Up and Arrow Down. |
| Home | Makes the first panel as small as its limits allow. |
| End | Makes the first panel as large as its limits allow. |
