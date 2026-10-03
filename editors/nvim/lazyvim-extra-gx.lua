-- LazyVim extra for Gx (REQ-TLS-06). Copy this file to
-- `lua/lazyvim/plugins/extras/lang/gx.lua` (a local extra) or import the
-- pieces into `lua/plugins/gx.lua`.
return {
  {
    "neovim/nvim-lspconfig",
    opts = function(_, opts)
      opts.servers = opts.servers or {}
      opts.servers.gx = {
        cmd = function()
          local bin = vim.env.GX_BIN or "gx"
          return { bin, "lsp", "--root", vim.fn.getcwd() }
        end,
        filetypes = { "gx" },
        root_markers = { "go.mod", ".git" },
      }
    end,
  },
  {
    "stevearc/conform.nvim",
    opts = function(_, opts)
      opts.formatters = opts.formatters or {}
      opts.formatters.gx = {
        command = "gx",
        args = { "fmt", "$FILENAME" },
        stdin = false,
      }
      opts.formatters_by_ft = opts.formatters_by_ft or {}
      opts.formatters_by_ft.gx = { "gx" }
    end,
  },
  {
    "nvim-treesitter/nvim-treesitter",
    opts = function(_, opts)
      opts.ensure_installed = opts.ensure_installed or {}
      vim.list_extend(opts.ensure_installed, { "gx", "go" })
    end,
  },
}
