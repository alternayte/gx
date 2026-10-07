package dev.alternayte.gx

import com.intellij.execution.Executor
import com.intellij.execution.configurations.CommandLineState
import com.intellij.execution.configurations.ConfigurationFactory
import com.intellij.execution.configurations.ConfigurationType
import com.intellij.execution.configurations.RunConfiguration
import com.intellij.execution.configurations.RunConfigurationBase
import com.intellij.execution.configurations.RunConfigurationOptions
import com.intellij.execution.configurations.RunProfileState
import com.intellij.execution.configurations.SimpleConfigurationType
import com.intellij.execution.process.KillableColoredProcessHandler
import com.intellij.execution.process.ProcessHandler
import com.intellij.execution.process.ProcessTerminatedListener
import com.intellij.execution.runners.ExecutionEnvironment
import com.intellij.icons.AllIcons
import com.intellij.openapi.options.SettingsEditor
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.NotNullLazyValue
import com.intellij.ui.components.JBTextField
import com.intellij.util.execution.ParametersListUtil
import com.intellij.util.ui.FormBuilder
import javax.swing.JComponent

/** The run configuration type "Gx dev": it runs `gx dev` in the project (REQ-TLS-07). */
class GxDevConfigurationType : SimpleConfigurationType(
    ID,
    "Gx dev",
    "Runs the Gx dev server: gx dev",
    NotNullLazyValue.createValue { AllIcons.Actions.Execute },
) {
    override fun createTemplateConfiguration(project: Project): RunConfiguration =
        GxDevRunConfiguration(project, this, "gx dev")

    override fun getOptionsClass(): Class<out RunConfigurationOptions> = GxDevOptions::class.java

    companion object {
        const val ID = "GxDevRunConfiguration"
    }
}

/** The stored options of one "Gx dev" configuration. */
class GxDevOptions : RunConfigurationOptions() {
    private val argumentsProperty = string("").provideDelegate(this, "arguments")

    /** Extra arguments of gx dev, for example -addr 127.0.0.1:8080. */
    var arguments: String
        get() = argumentsProperty.getValue(this).orEmpty()
        set(value) = argumentsProperty.setValue(this, value)
}

class GxDevRunConfiguration(project: Project, factory: ConfigurationFactory, name: String) :
    RunConfigurationBase<GxDevOptions>(project, factory, name) {

    override fun getOptions(): GxDevOptions = super.getOptions() as GxDevOptions

    var arguments: String
        get() = options.arguments
        set(value) {
            options.arguments = value
        }

    override fun getConfigurationEditor(): SettingsEditor<out RunConfiguration> = GxDevSettingsEditor()

    override fun getState(executor: Executor, environment: ExecutionEnvironment): RunProfileState =
        object : CommandLineState(environment) {
            override fun startProcess(): ProcessHandler {
                val args = arrayOf("dev") + ParametersListUtil.parse(arguments)
                val handler = KillableColoredProcessHandler(GxSettings.commandLine(project, *args))
                ProcessTerminatedListener.attach(handler)
                return handler
            }
        }
}

class GxDevSettingsEditor : SettingsEditor<GxDevRunConfiguration>() {
    private val arguments = JBTextField()

    override fun resetEditorFrom(configuration: GxDevRunConfiguration) {
        arguments.text = configuration.arguments
    }

    override fun applyEditorTo(configuration: GxDevRunConfiguration) {
        configuration.arguments = arguments.text.trim()
    }

    override fun createEditor(): JComponent =
        FormBuilder.createFormBuilder().addLabeledComponent("Arguments of gx dev:", arguments).panel
}
