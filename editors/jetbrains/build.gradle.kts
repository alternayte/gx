// The Gx plugin for GoLand and the other JetBrains IDEs (REQ-TLS-07): the
// language server `gx lsp`, the TextMate grammar of the VS Code extension
// and a run configuration for `gx dev`.
import org.jetbrains.intellij.platform.gradle.TestFrameworkType

plugins {
    id("org.jetbrains.kotlin.jvm") version "2.2.20"
    id("org.jetbrains.intellij.platform") version "2.19.0"
}

group = "dev.alternayte.gx"
version = "0.2.0"

kotlin {
    jvmToolchain(21)
}

// The plugin runs with the Kotlin library of the IDE. The oldest supported
// IDE (2025.1) has Kotlin 2.1.
tasks.named<org.jetbrains.kotlin.gradle.tasks.KotlinCompile>("compileKotlin") {
    compilerOptions {
        languageVersion.set(org.jetbrains.kotlin.gradle.dsl.KotlinVersion.KOTLIN_2_1)
        apiVersion.set(org.jetbrains.kotlin.gradle.dsl.KotlinVersion.KOTLIN_2_1)
    }
}

// The UI smoke test starts the IDE with the plugin (src/integrationTest).
sourceSets {
    create("integrationTest") {
        compileClasspath += sourceSets.main.get().output
        runtimeClasspath += sourceSets.main.get().output
    }
}

val integrationTestImplementation by configurations.getting {
    extendsFrom(configurations.testImplementation.get())
}

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        val idePath = providers.gradleProperty("gx.ide.path")
        if (idePath.isPresent) {
            local(idePath.get())
        } else {
            goland(providers.gradleProperty("gx.ide.version"))
        }
        bundledPlugin("org.jetbrains.plugins.textmate")
        testFramework(TestFrameworkType.Starter, configurationName = "integrationTestImplementation")
    }
    integrationTestImplementation("org.junit.jupiter:junit-jupiter:5.11.4")
    integrationTestImplementation("org.junit.platform:junit-platform-launcher:1.11.4")
    // The test framework needs the Kotlin library of its own version.
    integrationTestImplementation("org.jetbrains.kotlin:kotlin-stdlib:2.2.20")
    integrationTestImplementation("org.jetbrains.kotlin:kotlin-reflect:2.2.20")
    integrationTestImplementation("org.kodein.di:kodein-di-jvm:7.20.2")
    integrationTestImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-core-jvm:1.10.1")
}

intellijPlatform {
    pluginConfiguration {
        ideaVersion {
            sinceBuild = "251"
            untilBuild = provider { null }
        }
    }
}

// The grammar has one source: the VS Code extension. The plugin carries it
// as a TextMate bundle in the VS Code layout.
val textmateBundle = tasks.register<Copy>("textmateBundle") {
    from("../vscode") {
        include("syntaxes/gx.tmLanguage.json", "language-configuration.json")
    }
    from("src/main/textmate")
    into(layout.buildDirectory.dir("generated/textmate/textmate/gx"))
}

sourceSets {
    main {
        resources.srcDir(layout.buildDirectory.dir("generated/textmate"))
    }
}

tasks.processResources {
    dependsOn(textmateBundle)
}

val integrationTest by intellijPlatformTesting.testIdeUi.registering {
    task {
        val integrationTestSourceSet = sourceSets.getByName("integrationTest")
        testClassesDirs = integrationTestSourceSet.output.classesDirs
        classpath = integrationTestSourceSet.runtimeClasspath
        useJUnitPlatform()
        // scripts/jetbrains-smoke.sh gives the gx binary and the project.
        environment("GX_SERVER_PATH", providers.environmentVariable("GX_SERVER_PATH").getOrElse(""))
        systemProperty("gx.smoke.project", providers.environmentVariable("GX_SMOKE_PROJECT").getOrElse(""))
        // The test framework keeps its IDE data below this directory.
        systemProperty("gx.smoke.work", providers.environmentVariable("GX_SMOKE_WORK").getOrElse(layout.buildDirectory.dir("ide-tests").get().asFile.path))
        systemProperty("gx.ide.path", providers.gradleProperty("gx.ide.path").getOrElse(""))
        systemProperty("gx.ide.product", providers.gradleProperty("gx.ide.product").getOrElse("GO"))
        systemProperty("gx.ide.version", providers.gradleProperty("gx.ide.version").get())
        testLogging {
            events("passed", "failed")
            showStandardStreams = true
            exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
        }
    }
}
