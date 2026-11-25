package scaffold

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func testWrapperFS() fstest.MapFS {
	return fstest.MapFS{
		"templates/project/gradlew":                                  {Data: []byte("#!/bin/sh\necho gradlew\n")},
		"templates/project/gradlew.bat":                              {Data: []byte("@echo off\r\n")},
		"templates/project/gradle/wrapper/gradle-wrapper.jar":        {Data: []byte("jar-bytes")},
		"templates/project/gradle/wrapper/gradle-wrapper.properties": {Data: []byte("distributionUrl=https\\://services.gradle.org/distributions/gradle-default.zip\n")},
	}
}

func TestEnsureGradleWrapperRestoresMissingFilesAndPreservesProperties(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte("rootProject.name = \"App\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	propsDir := filepath.Join(dir, "gradle", "wrapper")
	if err := os.MkdirAll(propsDir, 0755); err != nil {
		t.Fatal(err)
	}
	customProps := "distributionUrl=https\\://services.gradle.org/distributions/gradle-9.3.1-bin.zip\n"
	if err := os.WriteFile(filepath.Join(propsDir, "gradle-wrapper.properties"), []byte(customProps), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGradleWrapper(testWrapperFS(), dir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "gradlew"))
	if err != nil {
		t.Fatalf("expected gradlew to be created: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("expected gradlew to be executable, mode=%v", info.Mode())
	}

	if _, err := os.Stat(filepath.Join(dir, "gradlew.bat")); err != nil {
		t.Fatalf("expected gradlew.bat to be created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gradle", "wrapper", "gradle-wrapper.jar")); err != nil {
		t.Fatalf("expected gradle-wrapper.jar to be created: %v", err)
	}

	gotProps, err := os.ReadFile(filepath.Join(propsDir, "gradle-wrapper.properties"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotProps) != customProps {
		t.Fatalf("expected existing gradle-wrapper.properties to be preserved, got %q", string(gotProps))
	}
}

func TestEnsureGradleWrapperRejectsNonGradleDir(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureGradleWrapper(testWrapperFS(), dir, true); err == nil {
		t.Fatal("expected error for non-Gradle directory")
	}
}
