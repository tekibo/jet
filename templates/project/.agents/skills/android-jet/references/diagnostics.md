# Diagnostics, in detail

`jet` is built to be read by machines: pass `--json` and read the JSON instead of parsing Gradle output. On any failure in `--json` mode, `jet` prints valid JSON to `stdout` and exits with status `1`.

## Compiler & Gradle build errors

```sh
jet build --json
```

On failure, `jet` returns:

```json
{
  "success": false,
  "command": "build",
  "error": "Execution failed for task ':app:validateSigningDebug': Keystore file '/path/debug.keystore' not found",
  "failed_tasks": [":app:validateSigningDebug"],
  "duration_ms": 4210,
  "diagnostics": [
    {
      "file": "app/src/main/kotlin/com/example/app/MainActivity.kt",
      "line": 42,
      "column": 15,
      "severity": "error",
      "kind": "kotlin",
      "task": ":app:compileDebugKotlin",
      "message": "Unresolved reference 'Button'",
      "suggestion": "Import 'androidx.compose.material3.Button'"
    }
  ],
  "raw_output_tail": "..."
}
```

- Inspect `diagnostics` first (`file`, `line`, `column`, `kind`, `task`, `message`, `suggestion`). Covers Kotlin (`kotlinc` / `.gradle.kts`), Java (`javac`), KSP, AAPT2 / resource errors, manifest merger, R8/D8, signing, and Gradle `* What went wrong:` task/dependency blocks.
- When a failure has no file/line (such as signing, dependency resolution, or unknown plugin errors), read `error`, `failed_tasks`, and `raw_output_tail` — `jet` never drops Gradle failure details.

## Test failures

```sh
jet test --json
```

Aggregates `TEST-*.xml` across all Gradle modules in the project:

```json
{
  "success": false,
  "command": "test",
  "error": "1 test(s) failed",
  "total": 12,
  "passed": 11,
  "failed": 1,
  "duration_ms": 3420,
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

If Gradle fails before tests execute (e.g., compile or configuration error), `test --json` includes `error`, `failed_tasks`, `diagnostics`, and `raw_output`.

## Other shapes

- `jet doctor --json`: `{"success": true, "command": "doctor", "data": {"has_errors": false, "sdk_path": "...", "checks": [...]}}`
- `jet run [dir] --json`: `{"success": true, "command": "run", "data": {"device": "...", "package": "...", "activity": "...", "component": "..."}}`
- `jet keystore --debug --json`: `{"success": true, "command": "keystore", "data": {"debug": true, "keystore": "debug.keystore", "alias": "androiddebugkey", "created": true}}`
