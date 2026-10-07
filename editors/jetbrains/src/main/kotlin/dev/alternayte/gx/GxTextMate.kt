package dev.alternayte.gx

import com.intellij.openapi.application.PathManager
import org.jetbrains.plugins.textmate.api.TextMateBundleProvider
import java.nio.file.Files
import java.nio.file.StandardCopyOption

/**
 * Gives the TextMate plugin the grammar of Gx (REQ-TLS-07). The bundle is
 * the grammar of the VS Code extension, in the VS Code layout. The TextMate
 * plugin reads a bundle from a directory, so the files of the plugin jar go
 * to the system directory of the IDE.
 */
class GxTextMateBundleProvider : TextMateBundleProvider {
    override fun getBundles(): List<TextMateBundleProvider.PluginBundle> {
        val dir = PathManager.getSystemDir().resolve("gx-textmate").resolve("gx")
        return try {
            for (name in FILES) {
                val target = dir.resolve(name)
                Files.createDirectories(target.parent)
                val source = javaClass.classLoader.getResourceAsStream("textmate/gx/$name") ?: return emptyList()
                source.use { Files.copy(it, target, StandardCopyOption.REPLACE_EXISTING) }
            }
            listOf(TextMateBundleProvider.PluginBundle("Gx", dir))
        } catch (e: java.io.IOException) {
            emptyList()
        }
    }

    companion object {
        val FILES = listOf("package.json", "language-configuration.json", "syntaxes/gx.tmLanguage.json")
    }
}
