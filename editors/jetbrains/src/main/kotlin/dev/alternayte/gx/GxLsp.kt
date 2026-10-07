package dev.alternayte.gx

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.platform.lsp.api.LspServerSupportProvider
import com.intellij.platform.lsp.api.ProjectWideLspServerDescriptor
import com.intellij.platform.lsp.api.customization.LspFormattingSupport

/**
 * Starts `gx lsp` when a .gx file opens (REQ-TLS-07). The IDE registers it
 * with the module "ultimate".
 */
open class GxLspServerSupportProvider : LspServerSupportProvider {
    override fun fileOpened(project: Project, file: VirtualFile, serverStarter: LspServerSupportProvider.LspServerStarter) {
        if (isGxFile(file)) {
            serverStarter.ensureServerStarted(GxLspServerDescriptor(project))
        }
    }
}

/** A service that exists only when the IDE has the module "ultimate". */
class GxUltimateLsp

/**
 * The same provider for the module "lsp". An IDE with both modules has both
 * providers, so this one does nothing there: one server serves a project.
 */
class GxLspModuleSupportProvider : GxLspServerSupportProvider() {
    override fun fileOpened(project: Project, file: VirtualFile, serverStarter: LspServerSupportProvider.LspServerStarter) {
        if (ApplicationManager.getApplication().getService(GxUltimateLsp::class.java) == null) {
            super.fileOpened(project, file, serverStarter)
        }
    }
}

/** One `gx lsp` process serves every .gx file of a project. */
class GxLspServerDescriptor(project: Project) : ProjectWideLspServerDescriptor(project, "Gx") {
    override fun isSupportedFile(file: VirtualFile): Boolean = isGxFile(file)

    override fun createCommandLine(): GeneralCommandLine = GxSettings.commandLine(project, "lsp")

    // Reformat Code is gx fmt: the IDE has no formatter of its own for a
    // .gx file. The property is the one that every supported IDE has.
    @Suppress("OVERRIDE_DEPRECATION", "DEPRECATION")
    override val lspFormattingSupport: LspFormattingSupport = object : LspFormattingSupport() {
        override fun shouldFormatThisFileExclusivelyByServer(
            file: VirtualFile,
            ideCanFormatThisFileItself: Boolean,
            serverExplicitlyWantsToFormatThisFile: Boolean,
        ): Boolean = isGxFile(file)
    }
}

fun isGxFile(file: VirtualFile): Boolean = file.extension == "gx"
