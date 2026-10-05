# Table

A data table built from composable parts.

## Usage

```gx
<table.Table>
  <table.TableHeader>
    <table.TableRow>
      <table.TableHead>Invoice</table.TableHead>
      <table.TableHead>Status</table.TableHead>
    </table.TableRow>
  </table.TableHeader>
  <table.TableBody>
    <table.TableRow>
      <table.TableCell>INV-001</table.TableCell>
      <table.TableCell>Paid</table.TableCell>
    </table.TableRow>
  </table.TableBody>
</table.Table>
```

For sorting, filtering and paging over many rows, use the data table component.

`TableFooter` holds the total rows. Set `data-state="selected"` on a `TableRow` to mark it.
A row that holds an expanded control takes the hover colour.
Pass `class="text-right"` to a `TableHead` and its cells for a number column.

## Do

- Put a `TableCaption` on every table.
- Use `TableHead` for header cells.

## Don't

- Do not use a table for layout.
- Do not put a table inside a table head.

## Keyboard

| Key | Action |
| --- | --- |
| None | The component is static. |
