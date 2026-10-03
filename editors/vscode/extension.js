// Gx VS Code extension (REQ-TLS-05). It starts `gx lsp` and wires the
// language features to VS Code. Plain CommonJS, no build step.
const vscode = require("vscode");
const { spawn } = require("child_process");
const path = require("path");

let client = null;
let output = null;

function activate(context) {
  output = vscode.window.createOutputChannel("Gx");
  context.subscriptions.push(output);

  client = new Client(context);
  context.subscriptions.push(client);

  context.subscriptions.push(
    vscode.commands.registerCommand("gx.restartServer", () => client.restart()),
    vscode.commands.registerCommand("gx.dev", () => {
      const task = new vscode.Task(
        { type: "gx", command: "dev" },
        vscode.TaskScope.Workspace,
        "dev",
        "gx",
        new vscode.ProcessExecution(serverPath(), ["dev"]),
        "$gx"
      );
      vscode.tasks.executeTask(task);
    })
  );

  context.subscriptions.push(
    vscode.tasks.registerTaskProvider("gx", {
      provideTasks: () => [
        new vscode.Task(
          { type: "gx", command: "dev" },
          vscode.TaskScope.Workspace,
          "dev",
          "gx",
          new vscode.ProcessExecution(serverPath(), ["dev"]),
          "$gx"
        ),
      ],
      resolveTask: (task) => task,
    })
  );

  context.subscriptions.push(
    vscode.languages.registerCompletionItemProvider("gx", {
      provideCompletionItems: (doc, pos) => client.completion(doc, pos),
    }),
    vscode.languages.registerHoverProvider("gx", {
      provideHover: (doc, pos) => client.hover(doc, pos),
    }),
    vscode.languages.registerDefinitionProvider("gx", {
      provideDefinition: (doc, pos) => client.definition(doc, pos),
    }),
    vscode.languages.registerDocumentFormattingEditProvider("gx", {
      provideDocumentFormattingEdits: (doc) => client.formatting(doc),
    }),
    vscode.languages.registerCodeActionsProvider(
      "gx",
      { provideCodeActions: (doc, range, ctx) => client.codeActions(doc, range, ctx) },
      { providedCodeActionKinds: [vscode.CodeActionKind.QuickFix, vscode.CodeActionKind.SourceOrganizeImports] }
    )
  );

  for (const doc of vscode.workspace.textDocuments) {
    client.openDocument(doc);
  }
  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument((doc) => client.openDocument(doc)),
    vscode.workspace.onDidChangeTextDocument((e) => client.changeDocument(e.document)),
    vscode.workspace.onDidCloseTextDocument((doc) => client.closeDocument(doc)),
    vscode.workspace.onDidSaveTextDocument((doc) => client.saveDocument(doc))
  );
}

function deactivate() {
  if (client) {
    client.dispose();
  }
}

// serverPath resolves the gx binary: GX_SERVER_PATH, then gx.server.path.
function serverPath() {
  const env = process.env.GX_SERVER_PATH;
  if (env) {
    return env;
  }
  const configured = vscode.workspace.getConfiguration("gx").get("server.path");
  return configured || "gx";
}

function workspaceRoot() {
  const folders = vscode.workspace.workspaceFolders;
  if (folders && folders.length > 0) {
    return folders[0].uri.fsPath;
  }
  return process.cwd();
}

class Client {
  constructor(context) {
    this.context = context;
    this.nextID = 1;
    this.pending = new Map();
    this.buffer = Buffer.alloc(0);
    this.docs = new Map();
    this.diagnostics = vscode.languages.createDiagnosticCollection("gx");
    context.subscriptions.push(this.diagnostics);
    this.start();
  }

  start() {
    const root = workspaceRoot();
    const bin = serverPath();
    output.appendLine(`starting ${bin} lsp --root ${root}`);
    this.proc = spawn(bin, ["lsp", "--root", root], { cwd: root, env: process.env });
    this.proc.on("error", (err) => output.appendLine(`lsp error: ${err.message}`));
    this.proc.on("exit", (code) => output.appendLine(`lsp exit: ${code}`));
    this.proc.stdout.on("data", (chunk) => this.read(chunk));
    this.proc.stderr.on("data", (chunk) => output.append(chunk.toString()));
    this.request("initialize", {
      processId: process.pid,
      rootUri: vscode.Uri.file(root).toString(),
      capabilities: {},
    }).catch((err) => output.appendLine(`initialize: ${err.message}`));
  }

  restart() {
    this.dispose();
    this.start();
  }

  dispose() {
    if (this.proc) {
      this.proc.kill();
      this.proc = null;
    }
  }

  // read consumes framed JSON-RPC messages.
  read(chunk) {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    for (;;) {
      const sep = this.buffer.indexOf("\r\n\r\n");
      if (sep < 0) {
        return;
      }
      const header = this.buffer.slice(0, sep).toString();
      const match = /Content-Length: (\d+)/.exec(header);
      if (!match) {
        this.buffer = this.buffer.slice(sep + 4);
        continue;
      }
      const length = Number(match[1]);
      if (this.buffer.length < sep + 4 + length) {
        return;
      }
      const body = this.buffer.slice(sep + 4, sep + 4 + length).toString();
      this.buffer = this.buffer.slice(sep + 4 + length);
      this.handle(JSON.parse(body));
    }
  }

  handle(msg) {
    if (msg.method === "textDocument/publishDiagnostics") {
      this.publish(msg.params);
      return;
    }
    if (msg.id !== undefined && msg.method === undefined) {
      const entry = this.pending.get(msg.id);
      if (!entry) {
        return;
      }
      this.pending.delete(msg.id);
      if (msg.error) {
        entry.reject(new Error(msg.error.message));
      } else {
        entry.resolve(msg.result);
      }
    }
  }

  publish(params) {
    const uri = vscode.Uri.parse(params.uri);
    const items = (params.diagnostics || []).map((d) => {
      const range = new vscode.Range(
        d.range.start.line, d.range.start.character,
        d.range.end.line, d.range.end.character
      );
      const item = new vscode.Diagnostic(range, d.message, vscode.DiagnosticSeverity.Error);
      item.code = d.code;
      item.source = d.source;
      return item;
    });
    this.diagnostics.set(uri, items);
  }

  request(method, params) {
    const id = this.nextID++;
    const body = JSON.stringify({ jsonrpc: "2.0", id, method, params });
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      const header = `Content-Length: ${Buffer.byteLength(body)}\r\n\r\n`;
      this.proc.stdin.write(header + body);
    });
  }

  notify(method, params) {
    const body = JSON.stringify({ jsonrpc: "2.0", method, params });
    this.proc.stdin.write(`Content-Length: ${Buffer.byteLength(body)}\r\n\r\n` + body);
  }

  openDocument(doc) {
    if (doc.languageId !== "gx") {
      return;
    }
    this.docs.set(doc.uri.toString(), doc);
    this.notify("textDocument/didOpen", {
      textDocument: { uri: doc.uri.toString(), languageId: "gx", version: doc.version, text: doc.getText() },
    });
  }

  changeDocument(doc) {
    if (doc.languageId !== "gx") {
      return;
    }
    this.docs.set(doc.uri.toString(), doc);
    this.notify("textDocument/didChange", {
      textDocument: { uri: doc.uri.toString(), version: doc.version },
      contentChanges: [{ text: doc.getText() }],
    });
  }

  closeDocument(doc) {
    if (doc.languageId !== "gx") {
      return;
    }
    this.docs.delete(doc.uri.toString());
    this.notify("textDocument/didClose", { textDocument: { uri: doc.uri.toString() } });
  }

  saveDocument(doc) {
    if (doc.languageId !== "gx") {
      return;
    }
    this.notify("textDocument/didSave", { textDocument: { uri: doc.uri.toString() } });
  }

  async completion(doc, pos) {
    const items = await this.request("textDocument/completion", {
      textDocument: { uri: doc.uri.toString() },
      position: { line: pos.line, character: pos.character },
    });
    if (!items || !items.items) {
      return [];
    }
    return items.items.map((raw) => {
      const item = new vscode.CompletionItem(raw.label, raw.kind || vscode.CompletionItemKind.Text);
      item.detail = raw.detail;
      if (raw.textEdit && raw.textEdit.range) {
        item.range = toRange(raw.textEdit.range);
        item.insertText = raw.textEdit.newText;
      }
      return item;
    });
  }

  async hover(doc, pos) {
    const result = await this.request("textDocument/hover", {
      textDocument: { uri: doc.uri.toString() },
      position: { line: pos.line, character: pos.character },
    });
    if (!result || !result.contents) {
      return null;
    }
    const value = typeof result.contents === "string" ? result.contents : result.contents.value;
    return new vscode.Hover(new vscode.MarkdownString(value), result.range ? toRange(result.range) : undefined);
  }

  async definition(doc, pos) {
    const result = await this.request("textDocument/definition", {
      textDocument: { uri: doc.uri.toString() },
      position: { line: pos.line, character: pos.character },
    });
    if (!result) {
      return null;
    }
    return result.map((loc) => new vscode.Location(vscode.Uri.parse(loc.uri), toRange(loc.range)));
  }

  async formatting(doc) {
    const edits = await this.request("textDocument/formatting", {
      textDocument: { uri: doc.uri.toString() },
      options: { tabSize: 2, insertSpaces: true },
    });
    if (!edits) {
      return [];
    }
    return edits.map((e) => new vscode.TextEdit(toRange(e.range), e.newText));
  }

  async codeActions(doc, range, context) {
    const result = await this.request("textDocument/codeAction", {
      textDocument: { uri: doc.uri.toString() },
      range: { start: { line: range.start.line, character: range.start.character }, end: { line: range.end.line, character: range.end.character } },
      context: {
        diagnostics: context.diagnostics.map((d) => ({ code: d.code, message: d.message })),
        only: context.only ? [context.only.value] : undefined,
      },
    });
    if (!result) {
      return [];
    }
    return result.map((raw) => {
      const action = new vscode.CodeAction(raw.title, kindOf(raw.kind));
      if (raw.edit && raw.edit.changes) {
        const edit = new vscode.WorkspaceEdit();
        for (const [uri, edits] of Object.entries(raw.edit.changes)) {
          for (const e of edits) {
            edit.replace(vscode.Uri.parse(uri), toRange(e.range), e.newText);
          }
        }
        action.edit = edit;
      }
      return action;
    });
  }
}

function toRange(range) {
  return new vscode.Range(range.start.line, range.start.character, range.end.line, range.end.character);
}

function kindOf(kind) {
  if (kind === "source.organizeImports") {
    return vscode.CodeActionKind.SourceOrganizeImports;
  }
  return vscode.CodeActionKind.QuickFix;
}

module.exports = { activate, deactivate };
