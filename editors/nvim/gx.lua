-- conform.nvim formatter for Gx (REQ-TLS-06). Merge into the conform setup:
--
--   require("conform").formatters.gx = require("gx").formatter
--   require("conform").formatters_by_ft.gx = { "gx" }
--
-- `gx fmt` rewrites the file in place, so conform must not feed stdin.
return {
  formatter = {
    command = "gx",
    args = { "fmt", "$FILENAME" },
    stdin = false,
  },
}
