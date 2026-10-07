package dev.alternayte.gx

import com.intellij.driver.client.Remote
import com.intellij.driver.client.service
import com.intellij.driver.sdk.invokeAction
import com.intellij.driver.sdk.openFile
import com.intellij.driver.sdk.singleProject
import com.intellij.driver.sdk.ui.components.common.codeEditor
import com.intellij.driver.sdk.ui.components.common.ideFrame
import com.intellij.driver.sdk.waitFor
import com.intellij.driver.sdk.waitForIndicators
import com.intellij.ide.starter.di.di
import com.intellij.ide.starter.driver.engine.runIdeWithDriver
import com.intellij.ide.starter.ide.IdeDownloader
import com.intellij.ide.starter.ide.IdeInstaller
import com.intellij.ide.starter.ide.IdeProductProvider
import com.intellij.ide.starter.ide.installer.ExistingIdeInstaller
import com.intellij.ide.starter.ide.installer.IdeInstallerFactory
import com.intellij.ide.starter.models.IdeInfo
import com.intellij.ide.starter.models.TestCase
import com.intellij.ide.starter.path.GlobalPaths
import com.intellij.ide.starter.plugins.PluginConfigurator
import com.intellij.ide.starter.project.LocalProjectInfo
import com.intellij.ide.starter.runner.Starter
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.kodein.di.DI
import org.kodein.di.bindSingleton
import java.nio.file.Path
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

/** The probe service of the plugin, called in the IDE process. */
@Remote("dev.alternayte.gx.GxProbe", plugin = "dev.alternayte.gx")
interface Probe {
    fun fileType(path: String): String
    fun grammarScope(name: String): String
    fun lspServers(): List<String>
    fun problems(path: String): List<String>
    fun text(path: String): String
    fun hasDevRunConfiguration(): Boolean
}

/** The files of the test framework go below the work directory, not into the repository. */
class WorkPaths(work: Path) : GlobalPaths(work)

/** The test starts the installed IDE when the build names one. */
class LocalIde(private val home: Path) : IdeInstallerFactory() {
    override fun createInstaller(ideInfo: IdeInfo, downloader: IdeDownloader): IdeInstaller = ExistingIdeInstaller(home)
}

/**
 * The UI smoke test of the plugin (REQ-TLS-07): the IDE starts with the
 * plugin, opens a project, highlights a .gx file, shows a diagnostic of
 * `gx lsp`, formats the file and has the "Gx dev" run configuration.
 * scripts/jetbrains-smoke.sh makes the project and the gx binary.
 */
class PluginSmokeTest {
    @Test
    fun openDiagnoseFormat() {
        val project = System.getProperty("gx.smoke.project").orEmpty()
        assertTrue(project.isNotEmpty(), "run the test with scripts/jetbrains-smoke.sh: GX_SMOKE_PROJECT is not set")
        val work = Path.of(System.getProperty("gx.smoke.work"))
        val idePath = System.getProperty("gx.ide.path").orEmpty()
        di = DI {
            extend(di)
            bindSingleton<GlobalPaths>(overrides = true) { WorkPaths(work) }
            if (idePath.isNotEmpty()) {
                bindSingleton<IdeInstallerFactory>(overrides = true) { LocalIde(Path.of(idePath)) }
            }
        }
        val ide = if (System.getProperty("gx.ide.product") == "GO") IdeProductProvider.GO else IdeProductProvider.IU
        var case = TestCase(ide, LocalProjectInfo(Path.of(project)))
        if (idePath.isEmpty()) case = case.withVersion(System.getProperty("gx.ide.version"))

        Starter.newContext("gxSmoke", case).apply {
            PluginConfigurator(this).installPluginFromPath(Path.of(System.getProperty("path.to.build.plugin")))
            applyVMOptionsPatch {
                addSystemProperty("idea.trust.all.projects", true)
                addSystemProperty("ide.show.tips.on.startup.default.value", false)
            }
        }.runIdeWithDriver().useDriverAndCloseIde {
            waitForIndicators(5.minutes)
            val probe = service<Probe>(singleProject())

            // Highlighting: the TextMate bundle of the plugin owns .gx files.
            assertEquals("source.gx", probe.grammarScope("Card.gx"))
            assertEquals("textmate", probe.fileType("card/Card.gx").lowercase())
            assertTrue(probe.hasDevRunConfiguration(), "no Gx dev run configuration type")

            // The language server starts when a .gx file opens, and its
            // diagnostic is a problem of the editor.
            openFile("card/Card.gx")
            waitFor("gx lsp runs", 2.minutes, 1.seconds) { probe.lspServers().any { it.equals("Running", ignoreCase = true) } }
            waitFor("the diagnostic of the misspelled prop", 2.minutes, 1.seconds) {
                probe.problems("card/Card.gx").any { it.contains("Titel") }
            }

            // Reformat Code formats through gx lsp.
            assertTrue(probe.text("card/Card.gx").contains("}\n\n\n<article>"), "the fixture is formatted before the test")
            ideFrame {
                // The action needs the editor as its context.
                invokeAction("ReformatCode", component = codeEditor().component)
            }
            waitFor("the formatted file", 1.minutes, 1.seconds) { probe.text("card/Card.gx").contains("}\n\n<article>") }
        }
    }
}
