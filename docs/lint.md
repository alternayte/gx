# Lint a Gx app with golangci-lint

`gx lint` runs `go vet` and the Gx analyzers over every package, generated
code included. The analyzers also ship as a golangci-lint module plugin, so
one grep finds app code and generated `_gx.go` code in the same run.

The analyzers report these codes.

| Code | Analyzer | Finding |
| --- | --- | --- |
| GX3005 | `gxroutepkg` | A route package holds code other than route types. |
| GX7001 | `gxsafehtml` | A `gx.SafeHTML` conversion of a non-constant value has no `//gx:trusted` on the same line. |

Generated files carry `//line` directives, so a finding in generated code
reports the `.gx` file and line (REQ-TLS-02).

## gx lint

```sh
gx lint        # the current module
gx lint ./app  # another module
```

## golangci-lint

The repo holds `.custom-gcl.yml`. Build the custom binary once.

```sh
golangci-lint custom
```

Build with a local checkout of this repository. The `path: .` entry tells
the builder to use the working tree, not a released version.

Then add the plugin config to the app's `.golangci.yml`:

```yaml
version: "2"

linters:
  default: standard
  enable:
    - gx
  settings:
    custom:
      gx:
        type: "module"
        description: Gx analyzers (route packages and safe HTML)
        settings: {}

  exclusions:
    # Generated _gx.go files carry the analyzers' findings.
    generated: disable
```

Run the custom binary in place of golangci-lint.

```sh
./bin/gx-golangci run ./...
```

`generated: disable` keeps generated `_gx.go` files in the analysis set.
With the default settings golangci-lint skips files that carry a generated
code header, and the `//line` mapped `.gx` findings disappear.
