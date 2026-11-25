package diagnostics

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ExitError indicates a command failed after already writing its structured
// output (such as JSON) to stdout, so main can exit non-zero without printing
// extra plain-text noise.
type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("command exited with status %d", e.Code)
}

func (e ExitError) Unwrap() error {
	return e.Err
}

type Diagnostic struct {
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Severity   string `json:"severity"` // "error" or "warning"
	Kind       string `json:"kind,omitempty"`
	Task       string `json:"task,omitempty"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type OutputEnvelope struct {
	Success       bool         `json:"success"`
	Command       string       `json:"command"`
	Error         string       `json:"error,omitempty"`
	FailedTasks   []string     `json:"failed_tasks,omitempty"`
	DurationMs    int64        `json:"duration_ms,omitempty"`
	Data          any          `json:"data,omitempty"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
	RawOutputTail string       `json:"raw_output_tail,omitempty"`
}

type GradleParseResult struct {
	Diagnostics   []Diagnostic
	FailedTasks   []string
	SummaryError  string
	RawOutputTail string
}

var (
	ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	taskHeaderRegex = regexp.MustCompile(`^>\s*Task\s+(:[^\s]+)(?:\s+(FAILED|UP-TO-DATE|FROM-CACHE|NO-SOURCE|SKIPPED))?`)
	execFailedRegex = regexp.MustCompile(`^Execution failed for task '([^']+)'\.?$`)
	scriptLocRegex  = regexp.MustCompile(`^(?:Build file|Script)\s+'([^']+)'\s+line:\s*(\d+)`)

	// Matches:
	// e: file:///path/file.kt:42:15: Unresolved reference: xyz
	// e: file:///path/file.kt:54:45 Unresolved reference 'xyz'
	// e: [ksp] file:///path/file.kt:42:15: ...
	// w: file:///path/file.kt:10:5: Warning message
	kotlinErrRegex1 = regexp.MustCompile(`^([ew]):\s+(?:\[([^\]]+)\]\s+)?(?:file://)?(.+?):(\d+):(\d+)(?::|\s)\s*(.*)$`)
	// Matches: e: /path/file.kt: (42, 15): Unresolved reference: xyz
	kotlinErrRegex2 = regexp.MustCompile(`^([ew]):\s+(?:\[([^\]]+)\]\s+)?(?:file://)?(.+?):\s*\((\d+),\s*(\d+)\)(?::|\s)\s*(.*)$`)
	// Matches: e: [ksp] /path/file.kt:42: message OR e: /path/file.kt:42: message
	kotlinErrRegex3 = regexp.MustCompile(`^([ew]):\s+(?:\[([^\]]+)\]\s+)?(?:file://)?(.+?):(\d+)(?::|\s)\s*(.*)$`)
	// Matches generic compiler lines: e: [ksp] message OR e: message
	kotlinGenericRegex = regexp.MustCompile(`^([ew]):\s+(?:\[([^\]]+)\]\s+)?(.+)$`)

	// Matches javac: /path/Foo.java:42: error: cannot find symbol
	javacErrRegex = regexp.MustCompile(`^(.+?\.java):(\d+):\s*(error|warning):\s*(.*)$`)

	// Matches AAPT2:
	// /path/res/layout/main.xml:12: AAPT: error: resource not found
	// ERROR: /path/res/values/strings.xml:12: AAPT: error: ...
	aaptErrRegex = regexp.MustCompile(`^(?:ERROR:\s*)?([^:\s][^:]*):(\d+)(?::(\d+))?:\s*AAPT:\s*(error|warning):\s*(.*)$`)
	// Matches AGP resource errors: ERROR: /path/file.xml: message
	agpResourceErrRegex = regexp.MustCompile(`^ERROR:\s*([^:\s]+\.xml)(?::(\d+))?:\s*(.+)$`)

	// Matches Gradle plugin version checks
	gradleVerRegex = regexp.MustCompile(`Minimum supported Gradle version is ([^.]+(?:\.[^.]+)*)\.\s*Current version is ([^.]+(?:\.[^.]+)*)`)
)

func ParseGradleOutput(output string) []Diagnostic {
	return ParseGradleResult(output).Diagnostics
}

func ParseGradleResult(output string) GradleParseResult {
	clean := cleanGradleText(output)
	lines := strings.Split(clean, "\n")

	var diagnostics []Diagnostic
	var failedTasks []string
	failedTaskSet := make(map[string]bool)
	diagSeen := make(map[string]bool)

	addFailedTask := func(task string) {
		task = strings.TrimSpace(task)
		if task != "" && !failedTaskSet[task] {
			failedTaskSet[task] = true
			failedTasks = append(failedTasks, task)
		}
	}

	addDiag := func(d Diagnostic) {
		d.File = cleanPath(d.File)
		d.Message = strings.TrimSpace(d.Message)
		if d.Message == "" {
			return
		}
		if d.Severity == "" {
			d.Severity = "error"
		}
		if d.Suggestion == "" {
			d.Suggestion = inferSuggestion(d.Message)
		}
		key := fmt.Sprintf("%s|%d|%d|%s|%s", d.File, d.Line, d.Column, d.Severity, d.Message)
		if diagSeen[key] {
			return
		}
		diagSeen[key] = true
		diagnostics = append(diagnostics, d)
	}

	var currentTask string
	inWhatWentWrong := false
	var wwwLines []string

	flushWhatWentWrong := func() {
		if len(wwwLines) == 0 {
			return
		}
		parsedWWW := parseWhatWentWrongBlock(wwwLines, diagnostics, addFailedTask)
		for _, d := range parsedWWW {
			addDiag(d)
		}
		wwwLines = nil
	}

	for i := 0; i < len(lines); i++ {
		rawLine := lines[i]
		line := strings.TrimSpace(rawLine)
		if line == "" {
			if inWhatWentWrong && len(wwwLines) > 0 {
				wwwLines = append(wwwLines, "")
			}
			continue
		}

		if strings.HasPrefix(line, "* What went wrong:") {
			inWhatWentWrong = true
			wwwLines = nil
			continue
		}

		if inWhatWentWrong {
			if strings.HasPrefix(line, "* Try:") ||
				strings.HasPrefix(line, "* Exception is:") ||
				strings.HasPrefix(line, "* Get more help") ||
				strings.HasPrefix(line, "BUILD FAILED") ||
				strings.HasPrefix(line, "Deprecated Gradle features") {
				flushWhatWentWrong()
				inWhatWentWrong = false
			} else {
				wwwLines = append(wwwLines, rawLine)
				continue
			}
		}

		if m := taskHeaderRegex.FindStringSubmatch(line); len(m) > 2 {
			currentTask = m[1]
			if m[2] == "FAILED" {
				addFailedTask(m[1])
			}
			continue
		}

		if m := kotlinErrRegex1.FindStringSubmatch(line); len(m) > 6 {
			l, _ := strconv.Atoi(m[4])
			c, _ := strconv.Atoi(m[5])
			addDiag(Diagnostic{
				File:     m[3],
				Line:     l,
				Column:   c,
				Severity: severityFromPrefix(m[1]),
				Kind:     kindFromTool(m[2], m[3]),
				Task:     currentTask,
				Message:  m[6],
			})
			continue
		}

		if m := kotlinErrRegex2.FindStringSubmatch(line); len(m) > 6 {
			l, _ := strconv.Atoi(m[4])
			c, _ := strconv.Atoi(m[5])
			addDiag(Diagnostic{
				File:     m[3],
				Line:     l,
				Column:   c,
				Severity: severityFromPrefix(m[1]),
				Kind:     kindFromTool(m[2], m[3]),
				Task:     currentTask,
				Message:  m[6],
			})
			continue
		}

		if m := kotlinErrRegex3.FindStringSubmatch(line); len(m) > 5 {
			l, _ := strconv.Atoi(m[4])
			addDiag(Diagnostic{
				File:     m[3],
				Line:     l,
				Severity: severityFromPrefix(m[1]),
				Kind:     kindFromTool(m[2], m[3]),
				Task:     currentTask,
				Message:  m[5],
			})
			continue
		}

		if m := kotlinGenericRegex.FindStringSubmatch(line); len(m) > 3 {
			addDiag(Diagnostic{
				Severity: severityFromPrefix(m[1]),
				Kind:     kindFromTool(m[2], ""),
				Task:     currentTask,
				Message:  m[3],
			})
			continue
		}

		if m := javacErrRegex.FindStringSubmatch(line); len(m) > 4 {
			addDiag(Diagnostic{
				File:     m[1],
				Line:     atoiOrZero(m[2]),
				Severity: strings.ToLower(m[3]),
				Kind:     "java",
				Task:     currentTask,
				Message:  m[4],
			})
			continue
		}

		if m := aaptErrRegex.FindStringSubmatch(line); len(m) > 5 {
			addDiag(Diagnostic{
				File:     m[1],
				Line:     atoiOrZero(m[2]),
				Column:   atoiOrZero(m[3]),
				Severity: strings.ToLower(m[4]),
				Kind:     "aapt",
				Task:     currentTask,
				Message:  m[5],
			})
			continue
		}

		if m := agpResourceErrRegex.FindStringSubmatch(line); len(m) > 3 {
			addDiag(Diagnostic{
				File:     m[1],
				Line:     atoiOrZero(m[2]),
				Severity: "error",
				Kind:     "aapt",
				Task:     currentTask,
				Message:  m[3],
			})
			continue
		}

		if m := gradleVerRegex.FindStringSubmatch(line); len(m) > 2 {
			addDiag(Diagnostic{
				Severity:   "error",
				Kind:       "gradle",
				Task:       currentTask,
				Message:    line,
				Suggestion: "Update distributionUrl in gradle/wrapper/gradle-wrapper.properties",
			})
			continue
		}

		if strings.Contains(line, "Manifest merger failed") {
			msg := line
			var details []string
			for j := i + 1; j < len(lines) && j <= i+10; j++ {
				next := strings.TrimSpace(lines[j])
				if next == "" || strings.HasPrefix(next, "> Task") || strings.HasPrefix(next, "*") {
					break
				}
				details = append(details, next)
			}
			if len(details) > 0 {
				msg = msg + ": " + strings.Join(details, " | ")
			}
			addDiag(Diagnostic{
				Severity: "error",
				Kind:     "manifest",
				Task:     currentTask,
				Message:  msg,
			})
			continue
		}

		if strings.HasPrefix(line, "Duplicate class ") || strings.Contains(line, "Missing classes detected while running R8") {
			addDiag(Diagnostic{
				Severity: "error",
				Kind:     "r8",
				Task:     currentTask,
				Message:  line,
			})
			continue
		}
	}

	if inWhatWentWrong {
		flushWhatWentWrong()
	}

	summary := buildSummaryError(diagnostics, failedTasks)
	return GradleParseResult{
		Diagnostics:   diagnostics,
		FailedTasks:   failedTasks,
		SummaryError:  summary,
		RawOutputTail: TailOutput(clean, 100, 12*1024),
	}
}

func parseWhatWentWrongBlock(lines []string, existing []Diagnostic, addFailedTask func(string)) []Diagnostic {
	var out []Diagnostic

	// Split by blank lines into sub-problems
	var blocks [][]string
	var cur []string
	for _, raw := range lines {
		t := strings.TrimSpace(raw)
		if t == "" {
			if len(cur) > 0 {
				blocks = append(blocks, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		blocks = append(blocks, cur)
	}

	for _, blk := range blocks {
		var taskName string
		var file string
		var lineNum int
		var parts []string

		for _, line := range blk {
			if m := execFailedRegex.FindStringSubmatch(line); len(m) > 1 {
				taskName = m[1]
				addFailedTask(taskName)
				continue
			}
			if m := scriptLocRegex.FindStringSubmatch(line); len(m) > 2 {
				file = m[1]
				lineNum = atoiOrZero(m[2])
				continue
			}

			cleaned := strings.TrimSpace(strings.TrimPrefix(line, ">"))
			if cleaned == "" {
				continue
			}
			parts = append(parts, cleaned)
		}

		msg := strings.Join(parts, ": ")
		if msg == "" && taskName != "" {
			msg = fmt.Sprintf("Execution failed for task '%s'", taskName)
		}
		if msg == "" {
			continue
		}

		// If Gradle merely says "Compilation error. See log for more details."
		// and we already extracted specific compiler diagnostics for this task (or overall),
		// avoid duplicating a vague summary diagnostic.
		if isGenericCompilationError(msg) && hasTaskOrCompilerDiagnostic(existing, taskName) {
			continue
		}

		if taskName != "" && !strings.Contains(msg, taskName) {
			msg = fmt.Sprintf("%s: %s", taskName, msg)
		}

		out = append(out, Diagnostic{
			File:       file,
			Line:       lineNum,
			Severity:   "error",
			Kind:       classifyFailureKind(msg, taskName),
			Task:       taskName,
			Message:    msg,
			Suggestion: inferSuggestion(msg),
		})
	}

	return out
}

func isGenericCompilationError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "compilation error. see log for more details") ||
		strings.Contains(lower, "compilation failed; see the compiler") ||
		strings.Contains(lower, "there were failing tests")
}

func hasTaskOrCompilerDiagnostic(diags []Diagnostic, taskName string) bool {
	for _, d := range diags {
		if d.Severity != "error" {
			continue
		}
		if taskName != "" && d.Task == taskName {
			return true
		}
		if d.Kind == "kotlin" || d.Kind == "java" || d.Kind == "ksp" || d.Kind == "aapt" || d.Kind == "manifest" {
			return true
		}
	}
	return false
}

func buildSummaryError(diags []Diagnostic, failedTasks []string) string {
	var errCount int
	var firstErr string
	for _, d := range diags {
		if d.Severity == "error" {
			errCount++
			if firstErr == "" {
				firstErr = d.Message
			}
		}
	}

	if errCount == 1 && firstErr != "" {
		return firstErr
	}
	if errCount > 1 {
		if firstErr != "" {
			return fmt.Sprintf("build failed with %d error(s): %s", errCount, firstErr)
		}
		return fmt.Sprintf("build failed with %d error(s)", errCount)
	}
	if len(failedTasks) > 0 {
		return fmt.Sprintf("build failed at task %s", strings.Join(failedTasks, ", "))
	}
	return "build failed"
}

func TailOutput(output string, maxLines, maxBytes int) string {
	clean := strings.TrimSpace(cleanGradleText(output))
	if clean == "" {
		return ""
	}
	lines := strings.Split(clean, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	joined := strings.Join(lines, "\n")
	if maxBytes > 0 && len(joined) > maxBytes {
		joined = joined[len(joined)-maxBytes:]
		if idx := strings.IndexByte(joined, '\n'); idx != -1 && idx+1 < len(joined) {
			joined = joined[idx+1:]
		}
	}
	return joined
}

func cleanGradleText(output string) string {
	s := strings.ReplaceAll(output, "\r\n", "\n")
	return ansiRegex.ReplaceAllString(s, "")
}

func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "file://")
	return p
}

func severityFromPrefix(p string) string {
	if strings.EqualFold(p, "w") {
		return "warning"
	}
	return "error"
}

func kindFromTool(tool, file string) string {
	if strings.EqualFold(tool, "ksp") {
		return "ksp"
	}
	if strings.HasSuffix(file, ".gradle.kts") {
		return "gradle"
	}
	return "kotlin"
}

func classifyFailureKind(msg, task string) string {
	lower := strings.ToLower(msg + " " + task)
	switch {
	case strings.Contains(lower, "keystore") || strings.Contains(lower, "signing"):
		return "signing"
	case strings.Contains(lower, "manifest merger"):
		return "manifest"
	case strings.Contains(lower, "aapt") || strings.Contains(lower, "resource"):
		return "aapt"
	case strings.Contains(lower, "could not resolve") || strings.Contains(lower, "could not find") || strings.Contains(lower, "dependencies"):
		return "dependency"
	case strings.Contains(lower, "ksp"):
		return "ksp"
	case strings.Contains(lower, "r8") || strings.Contains(lower, "proguard") || strings.Contains(lower, "duplicate class"):
		return "r8"
	default:
		return "gradle"
	}
}

func atoiOrZero(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func inferSuggestion(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "debug.keystore") && strings.Contains(lower, "not found"):
		return "Run 'jet keystore --debug' to generate debug.keystore, or check debug signingConfig in app/build.gradle(.kts)"
	case strings.Contains(lower, "keystore") && (strings.Contains(lower, "not found") || strings.Contains(lower, "password") || strings.Contains(lower, "signing")):
		return "Run 'jet keystore' or configure KEYSTORE_PATH, STORE_PASSWORD, KEY_PASSWORD, or keystore.properties"
	case strings.Contains(lower, "minimum supported gradle version"):
		return "Update distributionUrl in gradle/wrapper/gradle-wrapper.properties"
	case strings.Contains(lower, "license") && strings.Contains(lower, "not accepted"):
		return "Run 'yes | sdkmanager --licenses' and verify with 'jet doctor'"
	case strings.Contains(lower, "failed to find target with hash string") || strings.Contains(lower, "failed to install the following android sdk packages"):
		return "Run 'jet doctor' to verify installed Android SDK platforms and build-tools"
	case strings.Contains(lower, "namespace not specified"):
		return "Declare 'namespace = \"com.example.app\"' inside the android { ... } block"
	case strings.Contains(lower, "could not resolve") || strings.Contains(lower, "could not find"):
		return "Verify dependency coordinates/repositories in gradle/libs.versions.toml and settings.gradle.kts"
	case strings.Contains(msg, "Unresolved reference 'Button'") || strings.Contains(msg, "Unresolved reference: Button"):
		return "Import 'androidx.compose.material3.Button'"
	case strings.Contains(msg, "Unresolved reference 'Text'") || strings.Contains(msg, "Unresolved reference: Text"):
		return "Import 'androidx.compose.material3.Text'"
	case strings.Contains(msg, "Unresolved reference 'Modifier'") || strings.Contains(msg, "Unresolved reference: Modifier"):
		return "Import 'androidx.compose.ui.Modifier'"
	case strings.Contains(msg, "Unresolved reference 'Scaffold'") || strings.Contains(msg, "Unresolved reference: Scaffold"):
		return "Import 'androidx.compose.material3.Scaffold'"
	case strings.Contains(msg, "Unresolved reference 'Column'") || strings.Contains(msg, "Unresolved reference: Column"):
		return "Import 'androidx.compose.foundation.layout.Column'"
	case strings.Contains(msg, "Unresolved reference 'Row'") || strings.Contains(msg, "Unresolved reference: Row"):
		return "Import 'androidx.compose.foundation.layout.Row'"
	case strings.Contains(msg, "Unresolved reference 'Box'") || strings.Contains(msg, "Unresolved reference: Box"):
		return "Import 'androidx.compose.foundation.layout.Box'"
	case strings.Contains(msg, "Unresolved reference 'StateFlow'") || strings.Contains(msg, "Unresolved reference: StateFlow"):
		return "Import 'kotlinx.coroutines.flow.StateFlow'"
	case strings.Contains(msg, "Unresolved reference 'ViewModel'") || strings.Contains(msg, "Unresolved reference: ViewModel"):
		return "Import 'androidx.lifecycle.ViewModel'"
	default:
		return ""
	}
}
