<div align="center">

<img src="assets/logo.svg" width="96" height="96" alt="jet logo" />

# jet

<p><strong>Headless command-line toolchain for Android Jetpack Compose</strong></p>
<p>Scaffold, build, test, run, and sign modern Android applications from the terminal without Android Studio.</p>

<p>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Language-Go%201.23+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" /></a>
  <a href="https://developer.android.com/jetpack/compose"><img src="https://img.shields.io/badge/UI-Jetpack%20Compose-4285F4?style=flat-square&logo=android&logoColor=white" alt="Compose" /></a>
  <a href="https://github.com/tekibo/jet/releases"><img src="https://img.shields.io/badge/Platform-Linux%20|%20macOS%20|%20Windows-18181B?style=flat-square" alt="Platform" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" /></a>
</p>

</div>

---

## AI Agents

> Works with **codex**, **claude code**, **opencode**, **T3code**, **pi**, or your favorite agentic coding setup.

Copy and paste this prompt into your agent to build, run, and debug Android apps using `jet`:

```text
You are an expert native Android engineer using the jet CLI (https://github.com/tekibo/jet).

If jet is not installed on this machine, install it:
curl -sSL https://raw.githubusercontent.com/tekibo/jet/main/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"

Key workflows:
- Verify environment: `jet doctor --json`
- Compile & diagnostics: ALWAYS run `jet build --json` when fixing build errors to get clean JSON diagnostics (file, line, column, error) instead of reading raw Gradle logs.
- Scaffold: `jet init <folder> --name "Name" --package "com.pkg"`
- Presets: `jet add <nav|coil|icons|viewmodel|room|ktor|datastore|camera|work>`
- Code generation: `jet add-screen <Name>`, `jet add-viewmodel <Name>`
- Icon suite: `jet icon [file] [--bg <hex>]` (generates adaptive, monochrome, splash, and notification icons)
- Run on device: `jet run` (builds, installs, starts activity, and streams filtered logcat)
- Unit tests: `jet test --json`
- Reset cache: `jet clean --all`

Write pure Jetpack Compose UI (no XML), use edge-to-edge insets with Scaffold, and manage state using StateFlow. Adhere to the bundled skill at `.agents/skills/android-jet/SKILL.md`.
```

---

## Installation

Install `jet` to `~/.local/bin`:

```bash
curl -sSL https://raw.githubusercontent.com/tekibo/jet/main/install.sh | bash
```

### Install via Go

```bash
go install github.com/tekibo/jet@latest
```

---

## Commands

| Command | Description | Example |
| :--- | :--- | :--- |
| `jet init` | Scaffold a new Compose project | `jet init my-app` |
| `jet doctor` | Audit Android SDK, JDK, tools, and licenses | `jet doctor` |
| `jet run` | Build, install, launch activity, and stream logcat | `jet run my-app` |
| `jet build` | Build Debug/Release APK or App Bundle (.aab) | `jet build --release my-app` |
| `jet test` | Run unit tests with structured JSON reporting | `jet test my-app --json` |
| `jet icon` | Generate adaptive, monochrome, splash, & notification icons | `jet icon ./logo.png --bg "#1E88E5"` |
| `jet add` | Add preset library to Version Catalog and Gradle | `jet add coil my-app` |
| `jet devices` | List connected USB devices and running emulators | `jet devices` |
| `jet emu` | List or launch Android Virtual Devices | `jet emu start Pixel_8` |
| `jet add-screen` | Generate a Compose screen scaffold | `jet add-screen ProfileScreen` |
| `jet add-viewmodel` | Generate a UDF ViewModel with StateFlow | `jet add-viewmodel Profile` |
| `jet rename-pkg` | Rename package across files and directories | `jet rename-pkg --to com.co.app` |
| `jet keystore` | Generate release keystore and signing configuration | `jet keystore my-app` |
| `jet clean` | Clean build caches, stop daemons, and clear locks | `jet clean my-app --all` |

---

## Usage Examples

### 1. Run Loop (`jet run`)

Builds debug APK, installs to device/emulator, starts the launcher activity, and streams filtered logcat:

```bash
jet run my-app

# Or target a specific connected device
jet run my-app --device 393d7b52
```

### 2. Dependency Presets (`jet add`)

Adds dependencies and version coordinates into `gradle/libs.versions.toml` and `app/build.gradle.kts`:

```bash
jet add nav          # Jetpack Navigation Compose
jet add coil         # Coil 3 Image Loading
jet add icons        # Material 3 Extended Icons
jet add viewmodel    # Lifecycle ViewModel Compose
jet add room         # Room SQLite persistence
jet add ktor         # Ktor HTTP client
jet add datastore    # Jetpack DataStore Preferences
jet add camera       # AndroidX CameraX
jet add work         # WorkManager background jobs
```

### 3. Icon and Asset Generation (`jet icon`)

Generates the complete Android asset suite from any single PNG or SVG file:

```bash
# Auto-detects assets/icon/icon.svg or icon.png
jet icon my-app

# Or specify an image and background color
jet icon ./assets/logo.png --bg "#1E88E5" my-app
```

Generated assets:
- Adaptive foregrounds scaled to 66% safe zone (`mipmap-mdpi` through `xxxhdpi`).
- Android 13+ Material You monochrome themed icons (`ic_launcher_monochrome`).
- Android 12+ Core SplashScreen graphics (`ic_splash_logo`).
- Small notification icons (24dp pure white silhouette on transparent background).
- Legacy square and round fallback bitmaps.
- Play Store 512×512 icon (`app/playstore-icon.png`).

### 4. Tests and Diagnostics (`jet test` & `jet build`)

```bash
# Run unit tests and parse failure locations into JSON
jet test my-app --json

# Build debug APK with structured compiler error diagnostics
jet build my-app --json
```

### 5. Code Scaffolding (`jet add-screen` & `jet add-viewmodel`)

```bash
# Creates app/src/main/kotlin/<pkg>/ui/screens/HomeScreen.kt
jet add-screen HomeScreen my-app

# Creates app/src/main/kotlin/<pkg>/ui/viewmodels/HomeViewModel.kt
jet add-viewmodel Home my-app
```

### 6. Production Signing & Bundles (`jet keystore` & `jet build`)

```bash
# Generate production keystore and configure build.gradle.kts
jet keystore my-app

# Build signed release APK
jet build --release my-app

# Build Google Play Bundle (.aab)
jet build --aab my-app
```

---

## License

MIT
