# Breadcrumb

A trail of links to the current page.

## Usage

```gx
<breadcrumb.Breadcrumb>
  <breadcrumb.BreadcrumbList>
    <breadcrumb.BreadcrumbItem>
      <breadcrumb.BreadcrumbLink href={gx.URL("/")}>Home</breadcrumb.BreadcrumbLink>
    </breadcrumb.BreadcrumbItem>
    <breadcrumb.BreadcrumbSeparator />
    <breadcrumb.BreadcrumbItem>
      <breadcrumb.BreadcrumbPage>Products</breadcrumb.BreadcrumbPage>
    </breadcrumb.BreadcrumbItem>
  </breadcrumb.BreadcrumbList>
</breadcrumb.Breadcrumb>
```

## Do

- End the trail with `BreadcrumbPage`.
- Use `BreadcrumbEllipsis` for a collapsed middle.

## Don't

- Do not use a breadcrumb for primary navigation.
- Do not link the current page.

## Keyboard

Links follow the normal link keys. The components add no key bindings.
