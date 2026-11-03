---
name: android-jet
description: Build and manage native Android Jetpack Compose applications using the jet CLI without Android Studio. Use when creating, building, running, testing, or refactoring Android apps, adding dependencies, generating screens or ViewModels, creating icon suites, or diagnosing compiler errors.
license: MIT
compatibility: Linux, macOS, Windows. Requires OpenJDK 17+, Android SDK, bash, curl
metadata:
  author: tekibo
  version: "1.0.0"
allowed-tools: Read Write Bash(jet:*)
---

# Android Development with jet

`jet` is a command-line toolchain for building, running, and managing Android Jetpack Compose projects without Android Studio.

## Setup and Bootstrap

Before running Android tasks, ensure `jet` is installed:

```bash
command -v jet >/dev/null 2>&1 || curl -sSL https://raw.githubusercontent.com/tekibo/jet/main/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"
```

Check the environment:
```bash
jet doctor --json
```

If SDK packages or licenses are missing, install them:
```bash
yes | sdkmanager --licenses
sdkmanager "platform-tools" "platforms;android-36" "build-tools;36.0.0"
```

---

## Icon and Asset Generation

Modern Android applications require four distinct asset targets:
1. **Adaptive launcher icons**: 108x108 dp canvas with the central 72x72 dp (66%) safe zone.
2. **Material You themed icons (Android 13+)**: A `<monochrome>` silhouette layer tinted by system colors.
3. **Core SplashScreen API (Android 12+)**: 288x288 dp canvas with 192 dp inner circle (or 240 dp with 160 dp inner circle).
4. **Notification small icons**: 24x24 dp canvas (22x22 dp optical size), strictly pure white (`#FFFFFF`) on transparent background.

> [!NOTE]
> When designing custom icons or splash assets from scratch, search the web to verify current Android specifications for adaptive icon safe zones, Material You monochrome rules, and `windowSplashScreenAnimatedIcon` requirements.

### Standard Asset Directory
Place custom source assets in `assets/icon/`:
- `assets/icon/foreground.svg` or `foreground.png` (brand mark)
- `assets/icon/monochrome.svg` or `monochrome.png` (optional custom monochrome)
- `assets/icon/notification.svg` or `notification.png` (optional 24dp white silhouette)
- `assets/icon/splash.svg` or `splash.png` (optional 288dp splash asset)

### Generating Icons Without Source Images
When the user does not supply an icon, generate a clean geometric vector icon rather than leaving system placeholders:
1. Write `assets/icon/icon.svg` using standard SVG paths:
   ```xml
   <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">
     <path fill="#FFFFFFFF" d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
   </svg>
   ```
2. Select an appropriate brand hex color (e.g. `#1E88E5`).
3. Run `jet icon`:
   ```bash
   jet icon assets/icon/icon.svg --bg "#1E88E5" --json
   ```

### Stencil Monochrome Extraction
When given a single image (`jet icon logo.png`), `jet` uses luminance contrast thresholding (`Y = 0.299R + 0.587G + 0.114B`) to separate internal lines, letterforms, and cutouts from the background, avoiding solid black or white silhouettes.

---

## Compiler Diagnostics

Do not parse raw `./gradlew` terminal output for build errors. Run:
```bash
jet build --json
```

On failure, `jet` returns structured JSON:
```json
{
  "success": false,
  "command": "build",
  "error": "build failed with 1 error(s)",
  "diagnostics": [
    {
      "file": "/path/to/MainActivity.kt",
      "line": 42,
      "column": 15,
      "severity": "error",
      "message": "Unresolved reference 'Button'",
      "suggestion": "Import 'androidx.compose.material3.Button'"
    }
  ]
}
```

Inspect the `file`, `line`, and `column` directly to resolve errors.

---

## Testing and Cleaning

### Unit Tests
```bash
jet test --json
```
Parses JUnit XML results directly into JSON:
```json
{
  "success": false,
  "total": 12,
  "passed": 11,
  "failed": 1,
  "failures": [
    {
      "test_class": "com.example.app.UserViewModelTest",
      "test_name": "testStateFlowEmitsUser",
      "message": "expected:<Success> but was:<Loading>",
      "file": "UserViewModelTest.kt",
      "line": 48
    }
  ]
}
```

### Cache and Daemon Reset
```bash
jet clean --all --json
```
Stops lingering background Gradle daemons and removes `.gradle/`, `build/`, `.cxx/`, and lock files.

---

## Command Reference

Pass `--json` for machine-readable output:

| Action | Command | Purpose |
| :--- | :--- | :--- |
| Environment Check | `jet doctor --json` | Validates JDK, Android SDK, tools, and licenses |
| Scaffold | `jet init <dir> --name "Name" --package "com.pkg"` | Creates a Compose project with `.agents/skills/` |
| Devices | `jet devices --json` | Lists connected hardware devices and running emulators |
| Emulator | `jet emu start <avd_name> [--headless]` | Starts an Android Virtual Device |
| Run | `jet run [dir] [--device <serial>] [--no-logs]` | Builds debug APK, installs, launches activity, and streams logcat |
| Build | `jet build [--release] [--aab] [--json]` | Compiles app; outputs JSON diagnostics on failure |
| Test | `jet test [dir] [--json]` | Runs unit tests with parsed failure locations |
| Clean | `jet clean [dir] [--all] [--json]` | Cleans project caches and stops background daemons |
| Icons | `jet icon [file] [--bg <hex>] [--json]` | Generates adaptive, monochrome, splash, and notification icons |
| Dependencies | `jet add <preset> [dir]` | Presets: `nav`, `coil`, `icons`, `viewmodel`, `room`, `ktor`, `datastore`, `camera`, `work` |
| Screen | `jet add-screen <Name> [dir]` | Generates a Compose screen scaffold |
| ViewModel | `jet add-viewmodel <Name> [dir]` | Generates a UDF ViewModel with `StateFlow` |
| Rename Package | `jet rename-pkg [dir] --to <new.pkg>` | Refactors package name across source files and directories |
| Keystore | `jet keystore [dir]` | Generates production keystore and configures signing |

---

## Architecture Guidelines

1. **Jetpack Compose**: All UI is written as `@Composable` Kotlin functions without XML layouts.
2. **Edge-to-Edge**: `enableEdgeToEdge()` is active in `MainActivity.kt`. Consume WindowInsets using `Scaffold` inner padding:
   ```kotlin
   Scaffold(modifier = Modifier.fillMaxSize()) { innerPadding ->
       Content(modifier = Modifier.padding(innerPadding))
   }
   ```
3. **State Management**:
   - Expose UI state from ViewModels using `StateFlow`.
   - Avoid mutating state inside Composable bodies.
   - Run side-effects in `LaunchedEffect` or `rememberCoroutineScope`.
4. **Dependencies**: Declare versions in `gradle/libs.versions.toml`. Do not hardcode versions in `build.gradle.kts`.
