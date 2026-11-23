package diagnostics

import (
	"regexp"
	"strconv"
	"strings"
)

type Diagnostic struct {
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Severity   string `json:"severity"` // "error" or "warning"
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type OutputEnvelope struct {
	Success     bool          `json:"success"`
	Command     string        `json:"command"`
	Error       string        `json:"error,omitempty"`
	Data        any           `json:"data,omitempty"`
	Diagnostics []Diagnostic  `json:"diagnostics,omitempty"`
}

var (
	// Matches: e: file:///path/file.kt:42:15: Unresolved reference: xyz
	// Matches: e: file:///path/file.kt:54:45 Unresolved reference 'xyz'
	// Matches: e: /path/file.kt: (42, 15): Unresolved reference: xyz
	// Matches: w: file:///path/file.kt:10:5: Warning message
	kotlinErrRegex1 = regexp.MustCompile(`([ew]):\s+(?:file://)?([^:]+):(\d+):(\d+)(?::|\s)\s*(.*)`)
	kotlinErrRegex2 = regexp.MustCompile(`([ew]):\s+(?:file://)?([^:]+):\s*\((\d+),\s*(\d+)\)(?::|\s)\s*(.*)`)

	// Matches: /path/res/layout/main.xml:12: AAPT: error: resource not found
	aaptErrRegex = regexp.MustCompile(`([^:\s]+):(\d+):\s*AAPT:\s*(error|warning):\s*(.*)`)

	// Matches Gradle plugin version checks: Minimum supported Gradle version is X. Current version is Y.
	gradleVerRegex = regexp.MustCompile(`Minimum supported Gradle version is ([^.]+). Current version is ([^.]+)`)
)

func ParseGradleOutput(output string) []Diagnostic {
	var diagnostics []Diagnostic
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 1. Try Kotlin compiler format 1
		if matches := kotlinErrRegex1.FindStringSubmatch(line); len(matches) > 5 {
			severity := "error"
			if matches[1] == "w" {
				severity = "warning"
			}
			l, _ := strconv.Atoi(matches[3])
			c, _ := strconv.Atoi(matches[4])
			msg := strings.TrimSpace(matches[5])

			diag := Diagnostic{
				File:       cleanPath(matches[2]),
				Line:       l,
				Column:     c,
				Severity:   severity,
				Message:    msg,
				Suggestion: inferSuggestion(msg),
			}
			diagnostics = append(diagnostics, diag)
			continue
		}

		// 2. Try Kotlin compiler format 2: (line, col)
		if matches := kotlinErrRegex2.FindStringSubmatch(line); len(matches) > 5 {
			severity := "error"
			if matches[1] == "w" {
				severity = "warning"
			}
			l, _ := strconv.Atoi(matches[3])
			c, _ := strconv.Atoi(matches[4])
			msg := strings.TrimSpace(matches[5])

			diag := Diagnostic{
				File:       cleanPath(matches[2]),
				Line:       l,
				Column:     c,
				Severity:   severity,
				Message:    msg,
				Suggestion: inferSuggestion(msg),
			}
			diagnostics = append(diagnostics, diag)
			continue
		}

		// 3. Try AAPT2 resource error
		if matches := aaptErrRegex.FindStringSubmatch(line); len(matches) > 4 {
			l, _ := strconv.Atoi(matches[2])
			diagnostics = append(diagnostics, Diagnostic{
				File:     cleanPath(matches[1]),
				Line:     l,
				Severity: matches[3],
				Message:  strings.TrimSpace(matches[4]),
			})
			continue
		}

		// 4. Try Gradle wrapper version mismatch
		if matches := gradleVerRegex.FindStringSubmatch(line); len(matches) > 2 {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:   "error",
				Message:    line,
				Suggestion: "Update distributionUrl in gradle/wrapper/gradle-wrapper.properties",
			})
			continue
		}

		// 5. Manifest merger error
		if strings.Contains(line, "Manifest merger failed") {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: "error",
				Message:  line,
			})
			continue
		}
	}

	return diagnostics
}

func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "file://")
	return p
}

func inferSuggestion(msg string) string {
	if strings.Contains(msg, "Unresolved reference 'Button'") {
		return "Import 'androidx.compose.material3.Button'"
	}
	if strings.Contains(msg, "Unresolved reference 'Text'") {
		return "Import 'androidx.compose.material3.Text'"
	}
	if strings.Contains(msg, "Unresolved reference 'Modifier'") {
		return "Import 'androidx.compose.ui.Modifier'"
	}
	if strings.Contains(msg, "Unresolved reference 'Scaffold'") {
		return "Import 'androidx.compose.material3.Scaffold'"
	}
	if strings.Contains(msg, "Unresolved reference 'Column'") {
		return "Import 'androidx.compose.foundation.layout.Column'"
	}
	if strings.Contains(msg, "Unresolved reference 'Row'") {
		return "Import 'androidx.compose.foundation.layout.Row'"
	}
	if strings.Contains(msg, "Unresolved reference 'Box'") {
		return "Import 'androidx.compose.foundation.layout.Box'"
	}
	if strings.Contains(msg, "Unresolved reference 'StateFlow'") {
		return "Import 'kotlinx.coroutines.flow.StateFlow'"
	}
	if strings.Contains(msg, "Unresolved reference 'ViewModel'") {
		return "Import 'androidx.lifecycle.ViewModel'"
	}
	return ""
}
