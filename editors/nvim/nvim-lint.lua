-- nvim-lint linter for Gx (REQ-TLS-06). `gx lint` exits non-zero when it
-- finds an issue, so ignore_exitcode keeps the output.
local lint = require("lint")

return {
  gx = {
    cmd = "gx",
    args = { "lint" },
    stdin = false,
    stream = "stderr",
    ignore_exitcode = true,
    parser = lint.parser.from_pattern({
      pattern = "([^:]+):(%d+):(%d+): (%u+%d+): (.+)",
      groups = { "file", "lnum", "col", "code", "message" },
      source = "gx",
    }),
  },
}
