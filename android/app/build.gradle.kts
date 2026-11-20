import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// The release key: android/keystore.properties (storeFile, storePassword, keyAlias, keyPassword), not in the repository.
val keys = Properties().apply { rootProject.file("keystore.properties").takeIf { it.exists() }?.inputStream()?.use { load(it) } }

android {
    namespace = "com.henryvandenbroek.mardo"
    compileSdk = 36
    defaultConfig {
        applicationId = "com.henryvandenbroek.mardo"
        minSdk = 24
        targetSdk = 36
        versionCode = (project.findProperty("versionCode") as String? ?: "1").toInt()
        versionName = project.findProperty("versionName") as String? ?: "0.18.0"
    }
    signingConfigs {
        if (keys.isNotEmpty()) create("release") {
            storeFile = rootProject.file(keys.getProperty("storeFile"))
            storePassword = keys.getProperty("storePassword")
            keyAlias = keys.getProperty("keyAlias")
            keyPassword = keys.getProperty("keyPassword")
        }
    }
    buildTypes {
        release {
            isMinifyEnabled = false // the Go library and the page are not Java code to shrink
            if (keys.isNotEmpty()) signingConfig = signingConfigs.getByName("release")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }
}

// The page the app shows: frontend/dist-ios (npm run build:ios), copied into the app's assets.
val page by tasks.registering(Copy::class) {
    from(rootProject.file("../frontend/dist-ios"))
    into(layout.projectDirectory.dir("src/main/assets/dist"))
}
tasks.named("preBuild") { dependsOn(page) }

dependencies {
    implementation(files(rootProject.file("../build/mardo.aar"))) // the Go core: make android-aar
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("androidx.activity:activity-ktx:1.10.1")
    implementation("androidx.webkit:webkit:1.12.1")
}
