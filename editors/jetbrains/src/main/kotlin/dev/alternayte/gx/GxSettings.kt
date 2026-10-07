package dev.alternayte.gx

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.intellij.openapi.options.Configurable
import com.intellij.openapi.project.Project
import com.intellij.ui.components.JBTextField
import com.intellij.util.ui.FormBuilder
import java.io.File
import javax.swing.JComponent

/** The settings of the plugin: the gx command. */
@State(name = "GxSettings", storages = [Storage("gx.xml")])
class GxSettings : PersistentStateComponent<GxSettings.State> {
    class State {
        /** The path of the gx binary. Empty means the command of the project, or gx on the PATH. */
        var gxPath: String = ""
    }

    private var state = State()

    override fun getState(): State = state

    override fun loadState(state: State) {
        this.state = state
    }

    var gxPath: String
        get() = state.gxPath
        set(value) {
            state.gxPath = value
        }

    companion object {
        fun get(): GxSettings = ApplicationManager.getApplication().getService(GxSettings::class.java)

        /**
         * The command line of one gx command in a project. The order is: the
         * path of the settings, the GX_SERVER_PATH variable, the command of
         * the project (go run ./cmd/gx), and gx on the PATH.
         */
        fun commandLine(project: Project, vararg args: String): GeneralCommandLine {
            val dir = project.basePath
            val configured = get().gxPath.ifBlank { System.getenv("GX_SERVER_PATH").orEmpty() }
            val line = when {
                configured.isNotBlank() -> GeneralCommandLine(configured)
                dir != null && File(dir, "cmd/gx/main.go").isFile -> GeneralCommandLine("go", "run", "./cmd/gx")
                else -> GeneralCommandLine("gx")
            }
            line.addParameters(*args)
            if (dir != null) line.withWorkDirectory(dir)
            return line.withParentEnvironmentType(GeneralCommandLine.ParentEnvironmentType.CONSOLE)
        }
    }
}

/** Settings | Tools | Gx. */
class GxConfigurable : Configurable {
    private var field: JBTextField? = null

    override fun getDisplayName(): String = "Gx"

    override fun createComponent(): JComponent {
        val text = JBTextField(GxSettings.get().gxPath)
        text.emptyText.text = "go run ./cmd/gx in the project, or gx on the PATH"
        field = text
        return FormBuilder.createFormBuilder()
            .addLabeledComponent("Path of the gx binary:", text)
            .addComponentFillVertically(javax.swing.JPanel(), 0)
            .panel
    }

    override fun isModified(): Boolean = field?.text.orEmpty() != GxSettings.get().gxPath

    override fun apply() {
        GxSettings.get().gxPath = field?.text.orEmpty().trim()
    }

    override fun reset() {
        field?.text = GxSettings.get().gxPath
    }

    override fun disposeUIResources() {
        field = null
    }
}
