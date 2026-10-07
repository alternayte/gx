# Gx for GoLand and other JetBrains IDEs

The plugin gives a JetBrains IDE three things for a Gx app:

- Highlighting of `.gx` files. The plugin holds the TextMate grammar of the VS Code extension.
- The language server `gx lsp`: diagnostics, completion, hover, go to definition, rename and Reformat Code.
- The run configuration `Gx dev`, which runs `gx dev` in the project.

## Requirements

- An IDE of version 2025.1 or later.
- For the language server: an IDE with the LSP API. An IDE of version 2025.3 or later has it. An earlier IDE has it with a subscription. Highlighting and the run configuration work in every IDE.

## Build and install

```sh
./gradlew buildPlugin
```

The plugin is `build/distributions/gx-jetbrains-<version>.zip`. In the IDE, open Settings, then Plugins, then the gear menu, then "Install Plugin from Disk".

Gradle downloads GoLand to compile the plugin. To use an installed IDE, give its path:

```sh
./gradlew buildPlugin -Pgx.ide.path=/Applications/GoLand.app
```

## The gx command

The plugin finds the command in this order:

1. The path in Settings, then Tools, then Gx.
2. The `GX_SERVER_PATH` environment variable.
3. `go run ./cmd/gx` when the project has `cmd/gx/main.go`.
4. `gx` on the `PATH`.

## Test

`scripts/jetbrains-smoke.sh` in the repository root runs the UI smoke test. It starts the IDE with the plugin, opens a `.gx` file, waits for a diagnostic of `gx lsp`, and formats the file. The IDE needs a desktop session.

```sh
GX_IDE_PATH="/Applications/IntelliJ IDEA.app" GX_IDE_PRODUCT=IU scripts/jetbrains-smoke.sh
```

The version of the test framework must be equal to the build of the IDE. JetBrains publishes the framework for each public build.
