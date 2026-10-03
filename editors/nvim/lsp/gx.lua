-- Native Neovim LSP config for Gx (REQ-TLS-06). Neovim 0.11+ reads this file
-- from `lsp/gx.lua` on the runtimepath. Enable it with `vim.lsp.enable("gx")`.
-- GX_BIN overrides the binary path.
return {
  cmd = function()
    local bin = vim.env.GX_BIN or "gx"
    return { bin, "lsp", "--root", vim.fn.getcwd() }
  end,
  filetypes = { "gx" },
  root_markers = { "go.mod", ".git" },
  capabilities = {
    textDocument = {
      semanticTokens = {
        multilineTokenSupport = false,
      },
    },
  },
}
