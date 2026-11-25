# The jet CLI, in detail

Pass `--json` for machine-readable output on the commands that have it.

## Environment

- `jet doctor` - diagnose the local Android SDK & Java environment: JDK 17+, the Android SDK
  (platforms, build tools, platform tools) and licenses (`yes | sdkmanager --licenses`).
  With `--json`: `{"success": true, "command": "doctor", "data": {"has_errors": false, "sdk_path": "...", "checks": [{"name", "found", "warning", "description", "detail"}]}}`

## Project setup

- `jet init <dir> [--name "Name" --package "com.pkg"]` - scaffold a new Compose project.
- `jet add <preset> [dir]` - add a library to the Version Catalog and Gradle: nav, coil, icons,
  viewmodel, room, ktor, datastore, camera, work.
- `jet add-screen <Name> [dir]` - Compose screen scaffold.
- `jet add-viewmodel <Name> [dir]` - UDF ViewModel with StateFlow.

## Run and check

- `jet install [dir] [--device <serial>] [--module <mod>] [--apk <path>] [--build] [--release] [--reinstall] [--json]` - install an APK (builds first if missing or `--build` is set) without launching/streaming logcat; auto-picks a single connected device, lists & prompts interactively if multiple (or lists serials in `--json`), and generates v4 `.idsig` for standard debug APKs on Android 11+.
- `jet run [dir] [--device <serial>] [--module <mod>] [--reinstall] [--no-logs] [--json]` - build primary app module debug APK, install via `adb` (with v4 `.idsig` incremental install on Android 11+), detect and launch the launcher activity, and stream filtered logcat (or emit JSON when `--json` is passed). Use `--reinstall` to replace an existing install signed with a different key.
- `jet build [dir] [--release] [--aab] [--module <mod>] [--variant <var>] [--json]` - Debug APK (default), Release APK, or App Bundle across single- or multi-module projects.
- `jet test [dir] [--json]` - unit tests across all modules; JSON includes `failures` and pre-test Gradle `diagnostics` + `raw_output` on build failures.
- `jet clean [dir] [--all] [--json]` - clean build caches across modules, stop daemons, and clear locks.
- `jet icon [file] [--bg <hex>] [--module <mod>] [--splash-plate] [--json]` - generate the full icon suite (adaptive, monochrome, splash, notification) and sync `themes.xml` + `AndroidManifest.xml`.
- `jet rename-pkg [dir] [--to <new.pkg>]` - rename the package across files and manifests.
- `jet keystore [dir] [--debug] [--password <pw>] [--alias <a>] [--name <file>] [--force] [--json]` - generate a debug keystore (`--debug`) or release keystore & signing config non-interactively.
- `jet devices` - list connected hardware devices and running emulators.
- `jet emu start <avd_name> [--headless]` - start an Android Virtual Device.
