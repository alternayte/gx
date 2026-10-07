package editors

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestREQ_TLS_07_JetBrainsPlugin covers the sources of the JetBrains
// plugin: the language server through the LSP API, the TextMate bundle with
// the grammar of the VS Code extension, the "Gx dev" run configuration and
// the UI smoke test. scripts/jetbrains-smoke.sh runs the plugin in an IDE.
func TestREQ_TLS_07_JetBrainsPlugin(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "editors", "jetbrains")

	var plugin struct {
		ID      string `xml:"id"`
		Depends []struct {
			Optional string `xml:"optional,attr"`
			Config   string `xml:"config-file,attr"`
			ID       string `xml:",chardata"`
		} `xml:"depends"`
		Extensions []struct {
			NS    string `xml:"defaultExtensionNs,attr"`
			Items []struct {
				XMLName xml.Name
				Impl    string `xml:"implementation,attr"`
			} `xml:",any"`
		} `xml:"extensions"`
	}
	readXML(t, filepath.Join(dir, "src/main/resources/META-INF/plugin.xml"), &plugin)
	if plugin.ID != "dev.alternayte.gx" {
		t.Fatalf("plugin id = %q", plugin.ID)
	}
	required, lspConfigs := map[string]bool{}, map[string]string{}
	for _, d := range plugin.Depends {
		id := strings.TrimSpace(d.ID)
		if d.Optional == "true" {
			lspConfigs[id] = d.Config
		} else {
			required[id] = true
		}
	}
	// Highlighting needs the TextMate plugin. The LSP API is optional, so
	// the plugin loads in an IDE with no subscription.
	if !required["org.jetbrains.plugins.textmate"] || required["com.intellij.modules.ultimate"] {
		t.Fatalf("required dependencies = %v", required)
	}
	extensions := map[string]string{}
	collect := func(ns string, items []struct {
		XMLName xml.Name
		Impl    string `xml:"implementation,attr"`
	}) {
		for _, item := range items {
			extensions[ns+"."+item.XMLName.Local] = item.Impl
		}
	}
	for _, e := range plugin.Extensions {
		collect(e.NS, e.Items)
	}
	for _, module := range []string{"com.intellij.modules.ultimate", "com.intellij.modules.lsp"} {
		config := lspConfigs[module]
		if config == "" {
			t.Fatalf("no optional dependency on %s", module)
		}
		var part struct {
			Extensions []struct {
				NS    string `xml:"defaultExtensionNs,attr"`
				Items []struct {
					XMLName xml.Name
					Impl    string `xml:"implementation,attr"`
				} `xml:",any"`
			} `xml:"extensions"`
		}
		readXML(t, filepath.Join(dir, "src/main/resources/META-INF", config), &part)
		found := false
		for _, e := range part.Extensions {
			for _, item := range e.Items {
				if e.NS+"."+item.XMLName.Local == "com.intellij.platform.lsp.serverSupportProvider" {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("%s registers no LSP server support provider", config)
		}
	}
	if extensions["com.intellij.textmate.bundleProvider"] == "" {
		t.Fatalf("no TextMate bundle provider: %v", extensions)
	}
	if extensions["com.intellij.configurationType"] != "dev.alternayte.gx.GxDevConfigurationType" {
		t.Fatalf("no Gx dev run configuration type: %v", extensions)
	}

	kotlin := filepath.Join(dir, "src/main/kotlin/dev/alternayte/gx")
	for file, wants := range map[string][]string{
		"GxLsp.kt":      {"LspServerSupportProvider", `GxSettings.commandLine(project, "lsp")`, "shouldFormatThisFileExclusivelyByServer"},
		"GxDevRun.kt":   {`arrayOf("dev")`, "SimpleConfigurationType"},
		"GxTextMate.kt": {"TextMateBundleProvider", "syntaxes/gx.tmLanguage.json"},
		"GxSettings.kt": {`"go", "run", "./cmd/gx"`, "GX_SERVER_PATH"},
	} {
		body := readBody(t, filepath.Join(kotlin, file))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s lacks %q", file, want)
			}
		}
	}

	// One grammar: the build copies the file of the VS Code extension into
	// the bundle, and the bundle manifest names it.
	build := readBody(t, filepath.Join(dir, "build.gradle.kts"))
	for _, want := range []string{`from("../vscode")`, `"syntaxes/gx.tmLanguage.json"`, "testIdeUi"} {
		if !strings.Contains(build, want) {
			t.Fatalf("build.gradle.kts lacks %q", want)
		}
	}
	var bundle struct {
		Contributes struct {
			Languages []struct {
				Extensions []string `json:"extensions"`
			} `json:"languages"`
			Grammars []struct {
				Scope string `json:"scopeName"`
				Path  string `json:"path"`
			} `json:"grammars"`
		} `json:"contributes"`
	}
	readJSON(t, filepath.Join(dir, "src/main/textmate/package.json"), &bundle)
	if len(bundle.Contributes.Grammars) != 1 || bundle.Contributes.Grammars[0].Scope != "source.gx" ||
		bundle.Contributes.Grammars[0].Path != "./syntaxes/gx.tmLanguage.json" {
		t.Fatalf("bundle grammars = %+v", bundle.Contributes.Grammars)
	}
	if len(bundle.Contributes.Languages) != 1 || len(bundle.Contributes.Languages[0].Extensions) != 1 || bundle.Contributes.Languages[0].Extensions[0] != ".gx" {
		t.Fatalf("bundle languages = %+v", bundle.Contributes.Languages)
	}

	smoke := readBody(t, filepath.Join(dir, "src/integrationTest/kotlin/dev/alternayte/gx/PluginSmokeTest.kt"))
	for _, want := range []string{"runIdeWithDriver", `openFile("card/Card.gx")`, `invokeAction("ReformatCode"`, "hasDevRunConfiguration"} {
		if !strings.Contains(smoke, want) {
			t.Fatalf("the UI smoke test lacks %q", want)
		}
	}
	if info, err := os.Stat(filepath.Join(root, "scripts", "jetbrains-smoke.sh")); err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("scripts/jetbrains-smoke.sh is not an executable file: %v", err)
	}
}

func readXML(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
