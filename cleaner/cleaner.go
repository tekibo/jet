package cleaner

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"jet/diagnostics"
	"jet/scaffold"
)

type CleanResult struct {
	Success       bool     `json:"success"`
	Command       string   `json:"command"`
	CleanedPaths  []string `json:"cleaned_paths"`
	StoppedDaemon bool     `json:"stopped_daemon"`
	Error         string   `json:"error,omitempty"`
}

func RunClean(projectDir string, all bool, jsonMode bool, templatesFS fs.FS) error {
	if projectDir == "" {
		projectDir = "."
	}

	result := CleanResult{
		Success:      false,
		Command:      "clean",
		CleanedPaths: make([]string, 0),
	}

	fail := func(err error) error {
		if jsonMode {
			result.Error = err.Error()
			printJSON(result)
			return diagnostics.ExitError{Code: 1, Err: err}
		}
		return err
	}

	if err := scaffold.EnsureGradleWrapper(templatesFS, projectDir, jsonMode); err != nil {
		return fail(err)
	}

	if !jsonMode {
		fmt.Printf("[i] Cleaning project in '%s'...\n", projectDir)
	}

	cmd := exec.Command("./gradlew", "clean", "--console=plain")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		if !all {
			return fail(fmt.Errorf("gradle clean failed: %s", diagnostics.TailOutput(string(out), 40, 4096)))
		}
	}
	result.CleanedPaths = append(result.CleanedPaths, "gradle clean")

	if all {
		if !jsonMode {
			fmt.Println("  [i] Stopping background Gradle daemons...")
		}
		stopCmd := exec.Command("./gradlew", "--stop")
		stopCmd.Dir = projectDir
		_ = stopCmd.Run()
		result.StoppedDaemon = true

		targets := []string{
			filepath.Join(projectDir, "build"),
			filepath.Join(projectDir, ".gradle"),
			filepath.Join(projectDir, ".kotlin"),
			filepath.Join(projectDir, ".cxx"),
			filepath.Join(projectDir, ".externalNativeBuild"),
		}
		for _, modDir := range scaffold.FindModuleDirs(projectDir) {
			targets = append(targets,
				filepath.Join(modDir, "build"),
				filepath.Join(modDir, ".cxx"),
			)
		}

		seen := make(map[string]bool)
		for _, t := range targets {
			t = filepath.Clean(t)
			if seen[t] {
				continue
			}
			seen[t] = true
			if _, err := os.Stat(t); err == nil {
				_ = os.RemoveAll(t)
				rel, err := filepath.Rel(projectDir, t)
				if err != nil {
					rel = t
				}
				result.CleanedPaths = append(result.CleanedPaths, rel)
				if !jsonMode {
					fmt.Printf("  ✓ Removed %s\n", rel)
				}
			}
		}
	}

	result.Success = true

	if jsonMode {
		printJSON(result)
		return nil
	}

	fmt.Printf("[✓] Project cleaned successfully! (%d items cleared)\n", len(result.CleanedPaths))
	return nil
}

func printJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
