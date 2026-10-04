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
