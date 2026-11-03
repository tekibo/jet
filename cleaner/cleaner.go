package cleaner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type CleanResult struct {
	Success      bool     `json:"success"`
	Command      string   `json:"command"`
	CleanedPaths []string `json:"cleaned_paths"`
	StoppedDaemon bool    `json:"stopped_daemon"`
	Error        string   `json:"error,omitempty"`
}

func RunClean(projectDir string, all bool, jsonMode bool) error {
	if projectDir == "" {
		projectDir = "."
	}

	result := CleanResult{
		Success:      false,
		Command:      "clean",
		CleanedPaths: make([]string, 0),
	}

	gradlewPath := filepath.Join(projectDir, "gradlew")
	if _, err := os.Stat(gradlewPath); err != nil {
		err := fmt.Errorf("no gradlew found in '%s'", projectDir)
		if jsonMode {
			result.Error = err.Error()
			printJSON(result)
			return nil
		}
		return err
	}

	if !jsonMode {
		fmt.Printf("[i] Cleaning project in '%s'...\n", projectDir)
	}

	// 1. Run standard Gradle clean
	cmd := exec.Command(gradlewPath, "clean", "--console=plain")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		if !all {
			if jsonMode {
				result.Error = fmt.Sprintf("gradle clean failed: %s", string(out))
				printJSON(result)
				return nil
			}
			return fmt.Errorf("gradle clean failed: %s", string(out))
		}
	}
	result.CleanedPaths = append(result.CleanedPaths, "gradle clean (app/build)")

	// 2. If --all: stop daemons and wipe caches & locks
	if all {
		if !jsonMode {
			fmt.Println("  [i] Stopping background Gradle daemons...")
		}
		stopCmd := exec.Command(gradlewPath, "--stop")
		stopCmd.Dir = projectDir
		_ = stopCmd.Run()
		result.StoppedDaemon = true

		targets := []string{
			filepath.Join(projectDir, "build"),
			filepath.Join(projectDir, "app/build"),
			filepath.Join(projectDir, ".gradle"),
			filepath.Join(projectDir, ".cxx"),
			filepath.Join(projectDir, ".externalNativeBuild"),
		}

		for _, t := range targets {
			if _, err := os.Stat(t); err == nil {
				_ = os.RemoveAll(t)
				rel, _ := filepath.Rel(projectDir, t)
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
