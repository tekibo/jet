# Theming, Material You & Splash Alignment

All theme files live in `:core-ui` (`core-ui/src/main/kotlin/<pkg>/core/ui/`):
- `Color.kt`: full 36-token Light and Dark Material 3 fallback schemes (`Light*` / `Dark*`).
- `Type.kt`: `AppTypography` and `MonoCodeStyle`.
- `Shape.kt`: `AppShapes`.
- `Theme.kt`: `AppLightColorScheme`, `AppDarkColorScheme`, `resolveAppColorScheme`, and `AppTheme` (`MaterialExpressiveTheme` with `MotionScheme.expressive()`).
- `DynamicThemeSupportTest.kt` (`core-ui/src/test/...`): unit tests for Monet fallback detection.

## Tonal Harmony

- Keep `secondary` / `secondaryContainer` / `onSecondaryContainer` and `surfaceContainer*` tokens
  harmonious with the primary brand hue (slate-blue neutrals over blue/indigo surfaces in the
  template). Avoid discordant green/teal `secondary` accents against cool blue surfaces; reserve
  stronger hue contrast for `tertiary` or explicit semantic states.

## False-Positive Material You Guard (`isFallbackMonetConfiguration`)

Never trust `dynamicLightColorScheme` / `dynamicDarkColorScheme` blindly on Android 12+:
- OEM skins such as HyperOS set `Settings.Secure` `"theme_customization_overlay_packages"` to
  `"color_source":"preset"` with static fallback seed `#165C7D` even when users haven't enabled
  wallpaper theming.
- Un-overlaid AOSP emulators/ROMs similarly fall back to static seeds (`#00668B`/`#76D1FF` or
  `#445E91`/`#475D92`).

`resolveAppColorScheme(context, darkTheme, dynamicColor)` in `:core-ui`:
1. Allows system dynamic colors when `color_source` is `"home_wallpaper"` or `"lock_wallpaper"`,
   or a custom non-fallback preset.
2. Discards known OEM/AOSP fallback Monet presets as false positives and falls back cleanly to
   `AppLightColorScheme` / `AppDarkColorScheme`.
3. Memoizes on `LocalConfiguration.current` in `AppTheme` so wallpaper/configuration updates refresh
   without recreating schemes on every recomposition.

## Splash Background Continuity

- Keep `app/src/main/res/values/colors.xml` and `values-night/colors.xml` `splash_background` aligned
  with the app's light and dark window background so cold starts don't flash.
- Run `jet icon` (with `--splash-plate` for circular icon assets) to keep `values-v33/themes.xml` and
  `ic_splash_logo` safe-zone assets in sync (see `references/icons.md`).
