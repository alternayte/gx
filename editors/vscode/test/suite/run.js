// The smoke suite runs inside the VS Code extension host: open, diagnose,
// complete, format (REQ-TLS-05).
const vscode = require("vscode");
const path = require("path");

async function waitFor(fn, ms, what) {
  const deadline = Date.now() + ms;
  for (;;) {
    const value = fn();
    if (value) {
      return value;
    }
    if (Date.now() > deadline) {
      throw new Error("timeout waiting for " + what);
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
}

exports.run = async function () {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders || folders.length === 0) {
    throw new Error("no workspace folder");
  }
  const uri = vscode.Uri.file(path.join(folders[0].uri.fsPath, "card", "Card.gx"));
  const doc = await vscode.workspace.openTextDocument(uri);
  await vscode.window.showTextDocument(doc);

  // Open: the fixture holds a type error, so the server must diagnose it.
  const diags = await waitFor(() => {
    const found = vscode.languages.getDiagnostics(uri);
    return found.length > 0 ? found : null;
  }, 30000, "diagnostics");
  if (!diags.some((d) => d.code === "GX2000")) {
    throw new Error("no GX2000 in " + JSON.stringify(diags.map((d) => d.code)));
  }

  // Complete: the tag completion at `<`.
  const at = doc.positionAt(doc.getText().indexOf("<article") + 1);
  const items = await vscode.commands.executeCommand(
    "vscode.executeCompletionItemProvider",
    uri,
    at
  );
  if (!items || !items.items || items.items.length === 0) {
    throw new Error("no completions");
  }

  // Format: the fixture carries a double blank line.
  const edits = await vscode.commands.executeCommand(
    "vscode.executeFormatDocumentProvider",
    uri,
    { tabSize: 2, insertSpaces: true }
  );
  if (!edits || edits.length === 0) {
    throw new Error("no format edits");
  }

  console.log("gx vscode smoke: pass (diagnose, complete, format)");
};
