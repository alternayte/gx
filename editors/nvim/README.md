# Gx for Neovim

Native Neovim 0.11+ setup (REQ-TLS-06). Put `editors/nvim` on the
runtimepath, or copy `ftdetect/gx.lua` and `lsp/gx.lua` into your config.

```lua
vim.opt.runtimepath:prepend("/path/to/gx/editors/nvim")
vim.lsp.enable("gx")
```

`GX_BIN` overrides the binary path. The server is `gx lsp`.

- `gx.lua` holds the conform.nvim formatter.
- `nvim-lint.lua` holds the nvim-lint linter for `gx lint`.
- `lazyvim-extra-gx.lua` is a LazyVim extra.
- `tree-sitter-gx` holds the grammar, the highlight queries and the
  injection queries. Submit it to nvim-treesitter under `queries/gx/` with
  the parser from `editors/tree-sitter-gx`.

Run the headless smoke test with `just nvim-smoke`.
