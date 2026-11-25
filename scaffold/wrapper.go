package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type wrapperFile struct {
	tmplPath string
	destRel  string
	perm     os.FileMode
}

var wrapperFiles = []wrapperFile{
	{tmplPath: "templates/project/gradlew", destRel: "gradlew", perm: 0755},
	{tmplPath: "templates/project/gradlew.bat", destRel: "gradlew.bat", perm: 0644},
	{tmplPath: "templates/project/gradle/wrapper/gradle-wrapper.jar", destRel: filepath.Join("gradle", "wrapper", "gradle-wrapper.jar"), perm: 0644},
	{tmplPath: "templates/project/gradle/wrapper/gradle-wrapper.properties", destRel: filepath.Join("gradle", "wrapper", "gradle-wrapper.properties"), perm: 0644},
}

// EnsureGradleWrapper verifies gradlew and gradle-wrapper.jar exist in targetDir.
// If missing in a Gradle project, it restores the missing wrapper files from templatesFS
// while preserving any existing gradle-wrapper.properties.
func EnsureGradleWrapper(templatesFS fs.FS, targetDir string, quiet bool) error {
	if targetDir == "" {
		targetDir = "."
	}

	gradlewPath := filepath.Join(targetDir, "gradlew")
	jarPath := filepath.Join(targetDir, "gradle", "wrapper", "gradle-wrapper.jar")

	gradlewInfo, gradlewErr := os.Stat(gradlewPath)
	_, jarErr := os.Stat(jarPath)
	if gradlewErr == nil && jarErr == nil {
		if gradlewInfo.Mode()&0111 == 0 {
			_ = os.Chmod(gradlewPath, 0755)
		}
		return nil
	}

	if !isGradleProject(targetDir) {
		return fmt.Errorf("gradlew not found in %s", targetDir)
	}

	if templatesFS == nil {
		return fmt.Errorf("gradlew not found in %s", targetDir)
	}

	restored := false
	for _, wf := range wrapperFiles {
		destPath := filepath.Join(targetDir, wf.destRel)
		if _, err := os.Stat(destPath); err == nil {
			if wf.destRel == "gradlew" {
				_ = os.Chmod(destPath, wf.perm)
			}
			continue
		}

		data, err := fs.ReadFile(templatesFS, wf.tmplPath)
		if err != nil {
			return fmt.Errorf("failed to read embedded wrapper file %s: %w", wf.tmplPath, err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", destPath, err)
		}

		if err := os.WriteFile(destPath, data, wf.perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}
		restored = true
	}

	if restored && !quiet {
		fmt.Printf("[i] Gradle wrapper missing; restored gradlew in '%s'\n", targetDir)
	}

	return nil
}

func isGradleProject(dir string) bool {
	markers := []string{
		"settings.gradle.kts",
		"settings.gradle",
		"build.gradle.kts",
		"build.gradle",
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}
