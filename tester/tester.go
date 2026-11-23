package tester

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"jet/diagnostics"
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
	Total       int                      `json:"total"`
	Passed      int                      `json:"passed"`
	Failed      int                      `json:"failed"`
	Skipped     int                      `json:"skipped"`
	DurationMs  int64                    `json:"duration_ms"`
	Failures    []TestFailureDetail      `json:"failures,omitempty"`
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics,omitempty"`
	RawOutput   string                   `json:"raw_output,omitempty"`
}

func RunTests(projectDir string, jsonMode bool) error {
	if projectDir == "" {
		projectDir = "."
	}

	gradlewPath := filepath.Join(projectDir, "gradlew")
	if _, err := os.Stat(gradlewPath); err != nil {
		err := fmt.Errorf("no gradlew found in '%s'. Are you in an Android project root?", projectDir)
		if jsonMode {
			printReport(TestReport{
				Success: false,
				Command: "test",
				RawOutput: err.Error(),
			})
			return nil
		}
		return err
	}

	if !jsonMode {
		fmt.Printf("[i] Running unit tests in '%s'...\n", projectDir)
	}

	cmd := exec.Command(gradlewPath, "testDebugUnitTest", "--console=plain")
	cmd.Dir = projectDir

	outBytes, runErr := cmd.CombinedOutput()
	rawOutput := string(outBytes)

	// Check if compilation failed before tests could even run
	diags := diagnostics.ParseGradleOutput(rawOutput)
	if len(diags) > 0 {
		report := TestReport{
			Success:     false,
			Command:     "test",
			Diagnostics: diags,
			RawOutput:   rawOutput,
		}
		if jsonMode {
			printReport(report)
			return nil
		}
		fmt.Printf("\n[x] Compilation failed during test build with %d error(s):\n", len(diags))
		for _, d := range diags {
			fmt.Printf("  • %s:%d:%d: %s\n", d.File, d.Line, d.Column, d.Message)
		}
		return fmt.Errorf("tests failed to compile")
	}

	// Parse JUnit XML files
	testResultsDir := filepath.Join(projectDir, "app/build/test-results/testDebugUnitTest")
	report := parseTestResultsDir(testResultsDir)
	report.Command = "test"
	report.Success = (runErr == nil && report.Failed == 0)

	if jsonMode {
		printReport(report)
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

func parseTestResultsDir(dir string) TestReport {
	report := TestReport{
		Failures: make([]TestFailureDetail, 0),
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return report
	}

	stackRegex := regexp.MustCompile(`at\s+([a-zA-Z0-9_$.]+)\.([a-zA-Z0-9_]+)\(([a-zA-Z0-9_]+\.kt):(\d+)\)`)

	for _, f := range files {
		if !strings.HasPrefix(f.Name(), "TEST-") || !strings.HasSuffix(f.Name(), ".xml") {
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
			if tc.Failure != nil {
				failureDetail := TestFailureDetail{
					TestClass: tc.ClassName,
					TestName:  tc.Name,
					Message:   tc.Failure.Message,
				}

				// Extract exact file and line from stacktrace
				matches := stackRegex.FindAllStringSubmatch(tc.Failure.Content, -1)
				for _, m := range matches {
					if len(m) >= 5 {
						fileName := m[3]
						lineNum, _ := strconv.Atoi(m[4])
						failureDetail.File = fileName
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
