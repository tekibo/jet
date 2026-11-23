# Diagnostics, in detail

`jet` is built to be read by machines: pass `--json` and read the JSON instead of parsing Gradle output.

## Compiler errors

```sh
jet build --json
```

On failure, `jet` returns:

```json
{
  "success": false,
  "command": "build",
  "error": "build failed with 1 error(s)",
  "diagnostics": [
    {
      "file": "app/src/main/kotlin/com/example/app/MainActivity.kt",
      "line": 42,
      "column": 15,
      "severity": "error",
      "message": "Unresolved reference 'Button'",
      "suggestion": "Import 'androidx.compose.material3.Button'"
    }
  ]
}
```

Inspect `file`, `line`, and `column` - they point at the source to change.

## Test failures

```sh
jet test --json
```

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

## Other shapes

- `jet doctor --json`: `{"success": true, "command": "doctor", "data": {"has_errors": false, "sdk_path": "...", "checks": [...]}}`
