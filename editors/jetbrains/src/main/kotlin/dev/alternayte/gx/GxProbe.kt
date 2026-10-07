package dev.alternayte.gx

import com.intellij.codeInsight.daemon.impl.HighlightInfo
import com.intellij.execution.configurations.ConfigurationTypeUtil
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.application.ReadAction
import com.intellij.openapi.components.Service
import com.intellij.openapi.editor.Document
import com.intellij.openapi.editor.impl.DocumentMarkupModel
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.platform.lsp.api.LspServerManager
import org.jetbrains.plugins.textmate.TextMateService
import java.io.File

/**
 * Reports the state of the plugin in one project. The UI smoke test reads it
 * from outside the IDE (REQ-TLS-07); the plugin itself does not use it.
 */
@Service(Service.Level.PROJECT)
class GxProbe(private val project: Project) {
    /**
     * The name of the IDE file type of a file, by path below the project:
     * "textmate" for a .gx file. The TextMate plugin decides by file, not by
     * name.
     */
    fun fileType(path: String): String =
        LocalFileSystem.getInstance().refreshAndFindFileByIoFile(File(project.basePath, path))?.fileType?.name.orEmpty()

    /** The TextMate scope of a file name, or "" when no bundle has it. */
    fun grammarScope(name: String): String {
        val service = TextMateService.getInstance()
        if (service.getLanguageDescriptorByFileName(name) == null) return ""
        // The descriptor names its scope in a different way in each IDE
        // version; the map of file names to scopes is the same in all.
        return service.fileNameMatcherToScopeNameMapping.values.map { it.toString() }.firstOrNull { it == "source.gx" }.orEmpty()
    }

    /** The states of the `gx lsp` servers of the project. */
    fun lspServers(): List<String> =
        listOf(GxLspServerSupportProvider::class.java, GxLspModuleSupportProvider::class.java)
            .flatMap { LspServerManager.getInstance(project).getServersForProvider(it) }
            .map { it.state.name }

    /** The problems that the editor shows in a file, by path below the project. */
    fun problems(path: String): List<String> = ReadAction.compute<List<String>, RuntimeException> {
        val document = document(path) ?: return@compute emptyList()
        DocumentMarkupModel.forDocument(document, project, true).allHighlighters
            .mapNotNull { HighlightInfo.fromRangeHighlighter(it) }
            .filter { it.severity >= HighlightSeverity.WARNING }
            .mapNotNull { it.description }
    }

    /** The text of a file as the editor holds it. */
    fun text(path: String): String = ReadAction.compute<String, RuntimeException> { document(path)?.text.orEmpty() }

    /** Whether the "Gx dev" run configuration type is registered. */
    fun hasDevRunConfiguration(): Boolean = ConfigurationTypeUtil.findConfigurationType(GxDevConfigurationType.ID) != null

    private fun document(path: String): Document? {
        val file = LocalFileSystem.getInstance().findFileByIoFile(File(project.basePath, path)) ?: return null
        return FileDocumentManager.getInstance().getDocument(file)
    }
}
