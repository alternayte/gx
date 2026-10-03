// @vscode/test-electron entry point (REQ-TLS-05).
const path = require("path");
const { runTests } = require("@vscode/test-electron");

async function main() {
  const extensionDevelopmentPath = path.resolve(__dirname, "..");
  const extensionTestsPath = path.resolve(__dirname, "suite", "run.js");
  const fixture = process.env.GX_VSCODE_FIXTURE;
  const launchArgs = ["--disable-gpu", "--disable-extensions"];
  if (fixture) {
    launchArgs.push(fixture);
  }
  await runTests({
    extensionDevelopmentPath,
    extensionTestsPath,
    launchArgs,
    extensionTestsEnv: {
      GX_SERVER_PATH: process.env.GX_SERVER_PATH,
    },
  });
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
