package tester

import (
	"os"
	"path/filepath"
	"testing"

	"jet/scaffold"
)

func TestParseTestResultDirsMultiModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte(`include(":core-model", ":app")`), 0644); err != nil {
		t.Fatal(err)
	}

	for _, mod := range []string{"core-model", "app"} {
		if err := os.MkdirAll(filepath.Join(root, mod, "build", "test-results", "testDebugUnitTest"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	coreXML := `<testsuite name="CoreTest" tests="2" failures="0" errors="0" skipped="0" time="0.1"><testcase name="ok1" classname="CoreTest"/><testcase name="ok2" classname="CoreTest"/></testsuite>`
	appXML := `<testsuite name="AppTest" tests="2" failures="1" errors="0" skipped="0" time="0.2"><testcase name="ok" classname="AppTest"/><testcase name="bad" classname="AppTest"><failure message="boom">at com.example.AppTest.bad(AppTest.kt:27)</failure></testcase></testsuite>`

	if err := os.WriteFile(filepath.Join(root, "core-model", "build", "test-results", "testDebugUnitTest", "TEST-CoreTest.xml"), []byte(coreXML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "build", "test-results", "testDebugUnitTest", "TEST-AppTest.xml"), []byte(appXML), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := scaffold.FindTestResultDirs(root)
	report := parseTestResultDirs(dirs)
	if report.Total != 4 || report.Passed != 3 || report.Failed != 1 {
		t.Fatalf("unexpected report counts: %+v", report)
	}
	if len(report.Failures) != 1 || report.Failures[0].File != "AppTest.kt" || report.Failures[0].Line != 27 {
		t.Fatalf("unexpected failures: %+v", report.Failures)
	}
}
