---
title: "Diagnostics"
description: "Each diagnostic of gx check and gx lint has a stable code and a page."
section: Reference
order: 4
---

Each diagnostic has a code `GXnnnn`. The code does not change between releases. `gx check --json` and `gx lint --json` print the code, the position, the message, a link to the page and, where Gx knows one, a fix.

Each page shows the cause, an example that gives the diagnostic, and the fix.

## Files and generated code

| Code | Diagnostic |
| --- | --- |
| [GX1000](/errors/GX1000/) | Parse error. |
| [GX1001](/errors/GX1001/) | Component file name is not an exported identifier. |
| [GX1002](/errors/GX1002/) | Generated code is missing or stale. |

## Components and templates

| Code | Diagnostic |
| --- | --- |
| [GX2000](/errors/GX2000/) | Type error in an expression. |
| [GX2001](/errors/GX2001/) | Missing required prop. |
| [GX2002](/errors/GX2002/) | Unknown component. |
| [GX2003](/errors/GX2003/) | Unknown attribute or slot. |
| [GX2004](/errors/GX2004/) | Static value for a typed prop. |
| [GX2005](/errors/GX2005/) | Attribute spread on a component. |
| [GX2006](/errors/GX2006/) | Duplicate slot. |
| [GX2007](/errors/GX2007/) | Dynamic event attribute. |
| [GX2008](/errors/GX2008/) | Fragment free variable. |
| [GX2009](/errors/GX2009/) | Loop needs a key. |
| [GX2010](/errors/GX2010/) | Signal in a server expression. |
| [GX2011](/errors/GX2011/) | Dynamic URL attribute. |
| [GX2012](/errors/GX2012/) | Signal instance needs a key. |
| [GX2013](/errors/GX2013/) | Value cannot render as text. |
| [GX2014](/errors/GX2014/) | Signal needs an initial value. |
| [GX2015](/errors/GX2015/) | Component with signals has no top-level HTML element. |

## Routes

| Code | Diagnostic |
| --- | --- |
| [GX3000](/errors/GX3000/) | Route field type cannot bind. |
| [GX3001](/errors/GX3001/) | Pattern variable has no field. |
| [GX3002](/errors/GX3002/) | Path field has no pattern variable. |
| [GX3003](/errors/GX3003/) | Route value is not held by any gx.Collect. |
| [GX3004](/errors/GX3004/) | Duplicate route pattern. |
| [GX3005](/errors/GX3005/) | Route package contents. |

## Actions, signals and client expressions

| Code | Diagnostic |
| --- | --- |
| [GX4001](/errors/GX4001/) | No action is registered for a route type. |
| [GX4002](/errors/GX4002/) | More than one action is registered for a route type. |
| [GX4003](/errors/GX4003/) | No signal is declared for a signal-bound field. |
| [GX4004](/errors/GX4004/) | Signal type does not match the action field. |
| [GX4005](/errors/GX4005/) | Call is not allowed in a client expression. |
| [GX4007](/errors/GX4007/) | Value or operator is not allowed in a client expression. |
| [GX4008](/errors/GX4008/) | Signal-bound fields need rules or gx.Unchecked. |
| [GX4009](/errors/GX4009/) | Action method cannot be invoked from the client. |
| [GX4010](/errors/GX4010/) | Unknown event modifier or special event. |

## Styles and transitions

| Code | Diagnostic |
| --- | --- |
| [GX5001](/errors/GX5001/) | Gx.Enum misses a constant of its type. |
| [GX5002](/errors/GX5002/) | Duplicate view-transition-name in one template. |
| [GX5003](/errors/GX5003/) | Class string is built at runtime. |

## Islands

| Code | Diagnostic |
| --- | --- |
| [GX6001](/errors/GX6001/) | Island has no props struct. |
| [GX6002](/errors/GX6002/) | Island prop type has no TypeScript mapping. |

## Security

| Code | Diagnostic |
| --- | --- |
| [GX7001](/errors/GX7001/) | Conversion to gx.SafeHTML needs //gx:trusted. |
| [GX7002](/errors/GX7002/) | Gx.Secret cannot cross to the client. |

## Content

| Code | Diagnostic |
| --- | --- |
| [GX8001](/errors/GX8001/) | Frontmatter is malformed or unknown. |
| [GX8002](/errors/GX8002/) | Component is not declared in this collection. |
| [GX8003](/errors/GX8003/) | Content link or anchor is broken. |
| [GX8004](/errors/GX8004/) | Code file or line range is missing. |
