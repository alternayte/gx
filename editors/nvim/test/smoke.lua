-- Headless smoke test for the Neovim integration (REQ-TLS-06). Run through
-- scripts/nvim-smoke.sh: open a .gx file, attach the LSP, get diagnostics
-- and format the buffer.
local rtp = assert(vim.env.GX_NVIM_RTP, "GX_NVIM_RTP is not set")
local bin = assert(vim.env.GX_BIN, "GX_BIN is not set")
local fixture = assert(vim.env.GX_NVIM_FIXTURE, "GX_NVIM_FIXTURE is not set")

vim.opt.runtimepath:prepend(rtp)
vim.filetype.add({ extension = { gx = "gx" } })

vim.lsp.config("gx", {
  cmd = { bin, "lsp", "--root", vim.fn.getcwd() },
  filetypes = { "gx" },
  root_markers = { "go.mod" },
})
vim.lsp.enable("gx")

vim.cmd.edit(fixture)
vim.bo.filetype = "gx"

local attached = vim.wait(30000, function()
  return #vim.lsp.get_clients() > 0
end, 100)
assert(attached, "gx lsp did not attach")

local got = vim.wait(30000, function()
  return #vim.diagnostic.get(0) > 0
end, 100)
assert(got, "no diagnostics were published")
local found = false
for _, d in ipairs(vim.diagnostic.get(0)) do
  if d.code == "GX2000" then
    found = true
  end
end
assert(found, "no GX2000 diagnostic")

local before = table.concat(vim.api.nvim_buf_get_lines(0, 0, -1, false), "\n")
vim.lsp.buf.format({ async = false, timeout_ms = 30000 })
local after = table.concat(vim.api.nvim_buf_get_lines(0, 0, -1, false), "\n")
assert(after ~= before, "format changed nothing")

print("gx nvim smoke: pass (attach, diagnose, format)")
vim.cmd("qa!")
