---
title: "Forms"
description: "One rule set for the browser constraints, the live validation and the submit."
section: Guides
order: 4
---

A form input is a route type with a `Rules` method. The same rules give four checks: the browser constraints, the live validation of one field, the submit, and the form with no JavaScript.

```sh
go run ./cmd/gx new slice account
go run ./cmd/gx new form account/Signup
```

## The input and its rules

A rule points at a field with a pointer. When you rename the field, the rule and the view change with it or stop the build.

A nested struct binds with a dotted name, such as `address.city`. A file is a `gx.File` field.

```go title="account/route/route.go"
// Package route holds the route types of the account slice. A route
// package holds only route types.
package route

import "github.com/alternayte/gx"

// Index is the first page of the slice.
type Index struct {
	gx.Route `GET /account`
}

// SignupPage is the page that shows the signup form.
type SignupPage struct {
	gx.Route `GET /account/signup`
}

// Address is a nested part of the form.
type Address struct {
	City string
}

// Signup is the input of the signup form.
type Signup struct {
	gx.Route `POST /account/signup`
	Email    string
	Age      int
	Plan     string
	Terms    bool
	Avatar   gx.File
	Address  Address
}

// Rules holds the checks of the form.
func (in *Signup) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Email, gx.Required, gx.Email, gx.MaxLen(254)),
		gx.Field(&in.Age, gx.Min(18), gx.Max(120)),
		gx.Field(&in.Plan, gx.OneOf("free", "team")),
		gx.Field(&in.Terms, gx.True("terms.required")),
		gx.Field(&in.Avatar, gx.MaxSize(1<<20), gx.Accept("image/*")),
		gx.Field(&in.Address.City, gx.Required),
	}
}
```

## The rules

| Rule | Check | Browser attribute |
| --- | --- | --- |
| `gx.Required` | The value is not empty. | `required` |
| `gx.Email` | The value is an email address. | `type="email"` |
| `gx.IsURL` | The value is a URL. | `type="url"` |
| `gx.MinLen(n)`, `gx.MaxLen(n)` | The length of the text. | `minlength`, `maxlength` |
| `gx.Min(n)`, `gx.Max(n)` | The range of a number. | `min`, `max` |
| `gx.Pattern(re)` | The text matches a regular expression. | `pattern` |
| `gx.OneOf(values...)` | The value is one of a list. | none |
| `gx.True(key)` | A checkbox is on. | none |
| `gx.MaxSize(n)`, `gx.Accept(types...)` | The size and the type of a file. | `accept` |
| `gx.Each(rule)` | The rule passes for each item of a slice. | none |
| `gx.Check(fn)`, `gx.CheckCtx(fn)` | A function on the server. | none |

A rule with no browser attribute runs on the server only.

## The handler

`gx.Form(fn, view)` registers the form. The handler gets a pointer to the input, and it runs only when each rule passes.

```go title="account/signup_form.go"
package account

import (
	"github.com/alternayte/gx"
	"acme/account/route"
)

// Signup receives the signup form.
var Signup = gx.Form(func(c *gx.Ctx, in *route.Signup) error {
	if in.Email == "taken@example.com" {
		return gx.FieldError(&in.Email, "email.taken")
	}
	return c.Redirect(route.Index{})
}, SignupView)

// SignupPage shows the empty form.
var SignupPage = gx.Page(func(c *gx.Ctx, in route.SignupPage) (SignupViewProps, error) {
	return Signup.Props(&route.Signup{Plan: "free"}), nil
}, SignupView)
```

## The view

Gx generates `route.SignupForm` with one typed field for each input field. A field has `ID`, `Name`, `Value`, `Error` and `Attrs()`.

- `p.F.Attrs()` gives the form element its id, method, action and marker.
- `p.F.Email.Attrs()` gives a control its name, id, value, constraints and ARIA state.
- `data-gx-validate="blur"` or `"input"` checks one field on the server and patches only its error.

```gx title="account/SignupView.gx"
package account

import "acme/account/route"

props {
  // F is the typed form: one field per input field.
  F route.SignupForm
}

<gx.Head title="Sign up" />
<h1 class="text-2xl font-semibold">Sign up</h1>
<form {...p.F.Attrs()} class="mt-4 grid max-w-sm gap-2">
  <label for={p.F.Email.ID}>Email</label>
  <input {...p.F.Email.Attrs()} type="email" data-gx-validate="blur" class="rounded-md border border-border px-3 py-2" />
  <p id={p.F.Email.ID + "-error"} role="alert" class="text-sm text-destructive">{p.F.Email.Error}</p>

  <label for={p.F.Age.ID}>Age</label>
  <input {...p.F.Age.Attrs()} type="number" class="rounded-md border border-border px-3 py-2" />
  <p id={p.F.Age.ID + "-error"} role="alert" class="text-sm text-destructive">{p.F.Age.Error}</p>

  <label for={p.F.Plan.ID}>Plan</label>
  <select name={p.F.Plan.Name} id={p.F.Plan.ID} class="rounded-md border border-border px-3 py-2">
    <option value="free" selected={p.F.Plan.Value == "free"}>Free</option>
    <option value="team" selected={p.F.Plan.Value == "team"}>Team</option>
  </select>

  <label for={p.F.Address.City.ID}>City</label>
  <input {...p.F.Address.City.Attrs()} type="text" class="rounded-md border border-border px-3 py-2" />
  <p id={p.F.Address.City.ID + "-error"} role="alert" class="text-sm text-destructive">{p.F.Address.City.Error}</p>

  <label for={p.F.Avatar.ID}>Picture</label>
  <input {...p.F.Avatar.Attrs()} type="file" />
  <p id={p.F.Avatar.ID + "-error"} role="alert" class="text-sm text-destructive">{p.F.Avatar.Error}</p>

  <label class="flex items-center gap-2">
    <input {...p.F.Terms.Attrs()} type="checkbox" />
    I accept the terms
  </label>
  <p id={p.F.Terms.ID + "-error"} role="alert" class="text-sm text-destructive">{p.F.Terms.Error}</p>

  <button type="submit" class="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground">Create account</button>
</form>
```

```text title="GET /account/signup"
enctype="multipart/form-data"
name="address.city"
accept="image/*"
min="18"
```

A form with a file field gets `enctype="multipart/form-data"`. Gx checks the size before the handler runs, and it does not read a file that is too large into memory.

## The answers of a submit

| Case | With JavaScript | Without JavaScript |
| --- | --- | --- |
| A rule fails | Only the form element is patched. | Status 422 with the full page. |
| The handler returns `gx.FieldError` | The form shows that error. | The same, with status 422. |
| The handler redirects | The browser goes to the page. | Status 303. |

A value of the wrong type, such as text in the age field, is a field error with the key `invalid`. It is not a 400.

## Messages

A message is a key with an English default. `gx.SetTranslator` replaces the text for each key.

```go title="account/messages.go"
package account

import "github.com/alternayte/gx"

// UseShortMessages replaces two default messages.
func UseShortMessages() {
	gx.SetTranslator(func(key, fallback string) string {
		switch key {
		case "required":
			return "Required."
		case "email.taken":
			return "This email has an account."
		}
		return fallback
	})
}
```

## Repeated fields

A slice of structs binds with an index in the name, such as `addresses[0].city`. The generated field of the slice has an `Each` method that renders one row for each item. Add and remove a row with an action that patches the list.

## Cross-site requests

Each form and each action passes the cross-origin check of Go `net/http`. A browser that sends no Fetch Metadata needs a token, and the Gx runtime adds it.
