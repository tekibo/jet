# `:core-ui` Primitives, Modal Sheets & IME Handling

`:core-ui` (`<pkg>.core.ui`) provides reusable UI building blocks:
- `Primitives.kt`: `AppSurfaceCard`, `AppMetaPill`, `AppSectionHeader`, `AppEmptyState`, `AppTopBar` (icon parameters are slot lambdas `(@Composable () -> Unit)?` so `:core-ui` doesn't force extra icon dependencies).
- `Sheets.kt`: `AppModalSheet`, `AppTextField`, `dismissSheetKeyboard`, `dismissKeyboardOnEnter`.

## Why raw `ModalBottomSheet` + `BackHandler` fails on forms

Material 3 `ModalBottomSheet` renders inside its own `ComponentDialog` window with
`shouldDismissOnBackPress = true`. Under Android 13–16 predictive back or hardware keyboard input:
- An Activity-scoped Compose `BackHandler` attaches to the `Activity`'s `OnBackPressedDispatcher`,
  which the `ComponentDialog` bypasses.
- A Back gesture dismisses the entire sheet immediately and discards unsaved draft text instead of
  dismissing the keyboard/focus first.

## Two-Step Back & Keyboard Behavior in `AppModalSheet`

Use `AppModalSheet` instead of wiring raw `ModalBottomSheet` form sheets by hand:
1. Disables automatic dialog dismissal with `ModalBottomSheetProperties(shouldDismissOnBackPress = false)`.
2. Registers an `OnBackPressedCallback` on
   `LocalView.current.findViewTreeOnBackPressedDispatcherOwner()?.onBackPressedDispatcher`
   (falling back to `LocalOnBackPressedDispatcherOwner.current`), targeting the sheet's own Dialog
   dispatcher.
3. Pass `isInputFocused` tracked from `AppTextField`'s `onFocusChanged`:
   - **First Back (while `isInputFocused || WindowInsets.isImeVisible`)**: calls `dismissSheetKeyboard`
     (`focusManager.clearFocus(force = true)`, `keyboardController?.hide()`,
     `InputMethodManager.hideSoftInputFromWindow`, `view.clearFocus()`), keeping the sheet and
     draft open.
   - **Second Back (when unfocused and IME hidden)**: hides `sheetState` and invokes `onDismissRequest`.
4. Observes `WindowInsets.isImeVisible` transitions (`true -> false`) to clear stale text-field
   focus when the keyboard closes externally.

## `AppTextField` & Hardware Enter Dismissal

`AppTextField` styles inputs with `MaterialTheme.shapes.small` (and `MonoCodeStyle` when
`monospace = true`) and wires both:
- `KeyboardOptions(imeAction = imeAction)` + `KeyboardActions(onDone = ...)`
- `Modifier.dismissKeyboardOnEnter(...)` (intercepting hardware `Key.Enter` and `Key.NumPadEnter`)
  so pressing Enter on desktop/web/hardware keyboards dismisses focus and the IME cleanly on
  single-line `ImeAction.Done` fields.
