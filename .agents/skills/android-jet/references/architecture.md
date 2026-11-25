# Architecture guidelines

1. **Multi-module structure (`:app` + `:core-ui`)** - `jet init` scaffolds `:app`
   (`app/src/main/kotlin/<pkg>/`) alongside the `:core-ui` library module
   (`core-ui/src/main/kotlin/<pkg>/core/ui/`). `:app` and any future `:feature-*` modules import
   theme and reusable UI primitives from `<pkg>.core.ui` rather than redefining colors, typography,
   shapes, or sheet wrappers.
2. **Jetpack Compose** - all UI is `@Composable` Kotlin functions; no XML layouts.
3. **Edge-to-Edge** - `enableEdgeToEdge()` is active in `MainActivity.kt`. Wrap content in
   `AppTheme` and consume `WindowInsets` with `Scaffold` inner padding:
   ```kotlin
   Scaffold(modifier = Modifier.fillMaxSize()) { innerPadding ->
       Content(modifier = Modifier.padding(innerPadding))
   }
   ```
4. **State Management** - expose UI state from ViewModels using `StateFlow`; don't mutate state
   inside Composable bodies; run side effects in `LaunchedEffect` or `rememberCoroutineScope`.
5. **Dependencies** - declare versions in `gradle/libs.versions.toml` (or via `jet add <preset>`);
   never hardcode versions in `build.gradle.kts`. `:core-ui` primitives expose slot lambdas
   (`(@Composable () -> Unit)?`) so they compile cleanly without requiring `material-icons-extended`.
6. **Package renaming & scaffolding** - `jet rename-pkg` moves source trees and updates imports,
   namespaces, and manifests across all Gradle modules (`:app`, `:core-ui`, etc.); `jet add-screen`
   imports `:core-ui` primitives automatically when `:core-ui` exists.
7. **Specialists** - read `references/theming.md` before changing colors/dynamic theming,
   `references/sheets-and-ime.md` before building modal sheets or text inputs, and
   `references/icons.md` before changing launcher or splash assets.
