package diagnostics

import (
	"strings"
	"testing"
)

func TestParseGradleResultSigningFailure(t *testing.T) {
	out := `
> Task :app:compileDebugKotlin UP-TO-DATE
> Task :app:validateSigningDebug FAILED

FAILURE: Build failed with an exception.

* What went wrong:
Execution failed for task ':app:validateSigningDebug'.
> Keystore file '/tmp/app/debug.keystore' not found for signing config 'debugConfig'.

* Try:
> Run with --stacktrace option to get the stack trace.

BUILD FAILED in 16s
`
	res := ParseGradleResult(out)
	if len(res.FailedTasks) != 1 || res.FailedTasks[0] != ":app:validateSigningDebug" {
		t.Fatalf("unexpected failed tasks: %+v", res.FailedTasks)
	}
	if len(res.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}
	d := res.Diagnostics[0]
	if d.Kind != "signing" {
		t.Errorf("kind = %q, want signing", d.Kind)
	}
	if d.Task != ":app:validateSigningDebug" {
		t.Errorf("task = %q, want :app:validateSigningDebug", d.Task)
	}
	if !strings.Contains(d.Message, "debug.keystore") {
		t.Errorf("message %q missing debug.keystore", d.Message)
	}
	if d.Suggestion == "" {
		t.Error("expected suggestion for missing debug.keystore")
	}
	if !strings.Contains(res.SummaryError, "debug.keystore") {
		t.Errorf("summary = %q, want debug.keystore", res.SummaryError)
	}
}

func TestParseGradleResultKotlinDedupsGenericWhatWentWrong(t *testing.T) {
	out := `
> Task :app:compileDebugKotlin FAILED
e: file:///tmp/app/MainActivity.kt:42:15 Unresolved reference 'Button'
e: file:///tmp/app/MainActivity.kt:42:15 Unresolved reference 'Button'

FAILURE: Build failed with an exception.

* What went wrong:
Execution failed for task ':app:compileDebugKotlin'.
> A failure occurred while executing org.jetbrains.kotlin.compilerRunner.GradleCompilerRunnerWithWorkers$GradleKotlinCompilerRunnerAction
   > Compilation error. See log for more details.

* Try:
`
	res := ParseGradleResult(out)
	if len(res.Diagnostics) != 1 {
		t.Fatalf("expected exactly 1 deduplicated Kotlin diagnostic, got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}
	d := res.Diagnostics[0]
	if d.File != "/tmp/app/MainActivity.kt" || d.Line != 42 || d.Column != 15 {
		t.Errorf("unexpected location: %+v", d)
	}
	if d.Kind != "kotlin" {
		t.Errorf("kind = %q, want kotlin", d.Kind)
	}
	if !strings.Contains(d.Suggestion, "androidx.compose.material3.Button") {
		t.Errorf("unexpected suggestion: %q", d.Suggestion)
	}
}

func TestParseGradleResultJavacKspManifestAndDependency(t *testing.T) {
	out := `
> Task :core-data:kspDebugKotlin FAILED
e: [ksp] /tmp/app/Repo.kt:18: Missing @Inject constructor
> Task :core-model:compileDebugJavaWithJavac FAILED
/tmp/app/Model.java:10: error: cannot find symbol
> Task :app:processDebugMainManifest FAILED
Manifest merger failed : uses-sdk:minSdkVersion 21 cannot be smaller than version 24 declared in library
	Suggestion: use a compatible library with a minSdk of at most 21

* What went wrong:
A problem occurred configuring project ':app'.
> Could not resolve all artifacts for configuration ':app:debugRuntimeClasspath'.
   > Could not find com.foo:bar:1.2.3.

* Try:
`
	res := ParseGradleResult(out)
	if len(res.FailedTasks) != 3 {
		t.Fatalf("expected 3 failed tasks, got %+v", res.FailedTasks)
	}
	if len(res.Diagnostics) != 4 {
		t.Fatalf("expected 4 diagnostics, got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}
	if res.Diagnostics[0].Kind != "ksp" || res.Diagnostics[0].Line != 18 {
		t.Errorf("diag[0] = %+v", res.Diagnostics[0])
	}
	if res.Diagnostics[1].Kind != "java" || res.Diagnostics[1].Line != 10 {
		t.Errorf("diag[1] = %+v", res.Diagnostics[1])
	}
	if res.Diagnostics[2].Kind != "manifest" || !strings.Contains(res.Diagnostics[2].Message, "Suggestion:") {
		t.Errorf("diag[2] = %+v", res.Diagnostics[2])
	}
	if res.Diagnostics[3].Kind != "dependency" || !strings.Contains(res.Diagnostics[3].Message, "com.foo:bar:1.2.3") {
		t.Errorf("diag[3] = %+v", res.Diagnostics[3])
	}
}

func TestTailOutputBounds(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("line content\n")
	}
	tail := TailOutput(b.String(), 20, 10000)
	if gotLines := len(strings.Split(tail, "\n")); gotLines != 20 {
		t.Fatalf("got %d lines, want 20", gotLines)
	}
}
