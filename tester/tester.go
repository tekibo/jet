package tester

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jet/diagnostics"
	"jet/scaffold"
	"jet/signing"
)

type TestSuiteXML struct {
	XMLName   xml.Name      `xml:"testsuite"`
	Name      string        `xml:"name,attr"`
	Tests     int           `xml:"tests,attr"`
	Failures  int           `xml:"failures,attr"`
	Errors    int           `xml:"errors,attr"`
	Skipped   int           `xml:"skipped,attr"`
	Time      float64       `xml:"time,attr"`
	TestCases []TestCaseXML `xml:"testcase"`
}

type TestCaseXML struct {
	Name      string      `xml:"name,attr"`
	ClassName string      `xml:"classname,attr"`
	Time      float64     `xml:"time,attr"`
	Failure   *FailureXML `xml:"failure"`
	Error     *FailureXML `xml:"error"`
}

type FailureXML struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

type TestFailureDetail struct {
	TestClass string `json:"test_class"`
	TestName  string `json:"test_name"`
	Message   string `json:"message"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
}

type TestReport struct {
	Success     bool                     `json:"success"`
	Command     string                   `json:"command"`
	Error       string                   `json:"error,omitempty"`
	FailedTasks []string                 `json:"failed_tasks,omitempty"`
	Total       int                      `json:"total"`
	Passed      int                      `json:"passed"`
	Failed      int                      `json:"failed"`
	Skipped     int                      `json:"skipped"`
	DurationMs  int64                    `json:"duration_ms"`
	Failures    []TestFailureDetail      `json:"failures,omitempty"`
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics,omitempty"`
	RawOutput   string                   `json:"raw_output,omitempty"`
}

func RunTests(projectDir string, jsonMode bool, templatesFS fs.FS) error {
	if projectDir == "" {
		projectDir = "."
	}
	start := time.Now()

	failEarly := func(err error) error {
		if jsonMode {
			printReport(TestReport{
				Success:    false,
				Command:    "test",
				Error:      err.Error(),
				DurationMs: time.Since(start).Milliseconds(),
				RawOutput:  err.Error(),
			})
			return diagnostics.ExitError{Code: 1, Err: err}
		}
		return err
	}

	if err := scaffold.EnsureGradleWrapper(templatesFS, projectDir, jsonMode); err != nil {
		return failEarly(err)
	}

	if _, err := signing.EnsureDebugKeystore(projectDir, jsonMode); err != nil {
		return failEarly(err)
	}

	if !jsonMode {
		fmt.Printf("[i] Running unit tests in '%s'...\n", projectDir)
	}

	cmd := exec.Command("./gradlew", "testDebugUnitTest", "--console=plain")
	cmd.Dir = projectDir

	outBytes, runErr := cmd.CombinedOutput()
	rawOutput := string(outBytes)
	elapsedMs := time.Since(start).Milliseconds()

	parsed := diagnostics.ParseGradleResult(rawOutput)
	testResultDirs := scaffold.FindTestResultDirs(projectDir)
	report := parseTestResultDirs(testResultDirs)
	report.Command = "test"
	if report.DurationMs == 0 {
		report.DurationMs = elapsedMs
	}

	// If Gradle failed before any test results were recorded (or reported failures
	// came from compilation/configuration/signing), surface full structured diagnostics.
	if runErr != nil && report.Failed == 0 {
		report.Success = false
		report.FailedTasks = parsed.FailedTasks
		report.Diagnostics = parsed.Diagnostics
		report.RawOutput = parsed.RawOutputTail
		report.Error = parsed.SummaryError
		if report.Error == "" || report.Error == "build failed" {
			report.Error = fmt.Sprintf("test execution failed: %v", runErr)
		}

		if jsonMode {
			printReport(report)
			return diagnostics.ExitError{Code: 1, Err: runErr}
		}

		if len(parsed.Diagnostics) > 0 {
			fmt.Printf("\n[x] Test run failed with %d error(s):\n", len(parsed.Diagnostics))
			for _, d := range parsed.Diagnostics {
				if d.File != "" && d.Line > 0 {
					fmt.Printf("  • %s:%d:%d: %s\n", d.File, d.Line, d.Column, d.Message)
				} else {
					fmt.Printf("  • %s\n", d.Message)
				}
			}
		} else {
			fmt.Printf("\n[x] %s\n%s\n", report.Error, parsed.RawOutputTail)
		}
		return fmt.Errorf("%s", report.Error)
	}

	report.Success = (runErr == nil && report.Failed == 0)
	if !report.Success {
		report.Error = fmt.Sprintf("%d test(s) failed", report.Failed)
		if len(parsed.FailedTasks) > 0 {
			report.FailedTasks = parsed.FailedTasks
		}
	}

	if jsonMode {
		printReport(report)
		if !report.Success {
			return diagnostics.ExitError{Code: 1, Err: fmt.Errorf("%s", report.Error)}
		}
		return nil
	}

	if report.Success {
		fmt.Printf("\n[✓] All tests passed! (%d total, %d passed, %d skipped in %dms)\n",
			report.Total, report.Passed, report.Skipped, report.DurationMs)
		return nil
	}

	fmt.Printf("\n[x] %d test failure(s) detected out of %d tests:\n\n", report.Failed, report.Total)
	for _, f := range report.Failures {
		loc := ""
		if f.File != "" && f.Line > 0 {
			loc = fmt.Sprintf(" (%s:%d)", f.File, f.Line)
		}
		fmt.Printf("  • %s > %s%s\n", f.TestClass, f.TestName, loc)
		if f.Message != "" {
			fmt.Printf("    Message: %s\n", f.Message)
		}
	}
	return fmt.Errorf("%d test(s) failed", report.Failed)
}

func parseTestResultDirs(dirs []string) TestReport {
	report := TestReport{
		Failures: make([]TestFailureDetail, 0),
	}
	stackRegex := regexp.MustCompile(`at\s+([a-zA-Z0-9_$.]+)\.([a-zA-Z0-9_]+)\(([a-zA-Z0-9_]+\.(?:kt|java)):(\d+)\)`)

	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasPrefix(f.Name(), "TEST-") || !strings.HasSuffix(f.Name(), ".xml") {
				continue
			}

			xmlPath := filepath.Join(dir, f.Name())
			data, err := os.ReadFile(xmlPath)
			if err != nil {
				continue
			}

			var suite TestSuiteXML
			if err := xml.Unmarshal(data, &suite); err != nil {
				continue
			}

			report.Total += suite.Tests
			report.Failed += (suite.Failures + suite.Errors)
			report.Skipped += suite.Skipped
			report.DurationMs += int64(suite.Time * 1000)

			for _, tc := range suite.TestCases {
				fail := tc.Failure
				if fail == nil {
					fail = tc.Error
				}
				if fail == nil {
					continue
				}

				msg := strings.TrimSpace(fail.Message)
				if msg == "" {
					msg = strings.TrimSpace(fail.Type)
				}
				failureDetail := TestFailureDetail{
					TestClass: tc.ClassName,
					TestName:  tc.Name,
					Message:   msg,
				}

				matches := stackRegex.FindAllStringSubmatch(fail.Content, -1)
				for _, m := range matches {
					if len(m) >= 5 {
						lineNum, _ := strconv.Atoi(m[4])
						failureDetail.File = m[3]
						failureDetail.Line = lineNum
						break
					}
				}

				report.Failures = append(report.Failures, failureDetail)
			}
		}
	}

	report.Passed = report.Total - report.Failed - report.Skipped
	if report.Passed < 0 {
		report.Passed = 0
	}
	return report
}

func printReport(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
