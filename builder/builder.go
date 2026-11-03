package builder

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"jet/diagnostics"
)

type BuildOptions struct {
	TargetDir string
	Release   bool
	AAB       bool
	JSON      bool
}

type ArtifactInfo struct {
	Type      string  `json:"type"`
	File      string  `json:"file"`
	SizeBytes int64   `json:"size_bytes"`
	SizeMB    float64 `json:"size_mb"`
	SHA256    string  `json:"sha256"`
}

func BuildApp(opts BuildOptions) error {
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}

	gradlew := filepath.Join(opts.TargetDir, "gradlew")
	if _, err := os.Stat(gradlew); err != nil {
		if opts.JSON {
			outputJSON(diagnostics.OutputEnvelope{
				Success: false,
				Command: "build",
				Error:   fmt.Sprintf("gradlew not found in %s", opts.TargetDir),
			})
			return nil
		}
		return fmt.Errorf("gradlew not found in %s", opts.TargetDir)
	}

	task := "assembleDebug"
	artifactType := "Debug APK"
	if opts.AAB {
		task = "bundleRelease"
		artifactType = "Release Android App Bundle (.aab)"
	} else if opts.Release {
		task = "assembleRelease"
		artifactType = "Release APK"
	}

	if !opts.JSON {
		fmt.Printf("[i] Building %s via task ':%s'...\n", artifactType, task)
	}

	cmd := exec.Command("./gradlew", task)
	cmd.Dir = opts.TargetDir

	var outBuf bytes.Buffer
	if opts.JSON {
		// Capture output for error parsing
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	runErr := cmd.Run()
	if runErr != nil {
		if opts.JSON {
			outputStr := outBuf.String()
			diags := diagnostics.ParseGradleOutput(outputStr)
			errMsg := "build failed"
			if len(diags) > 0 {
				errMsg = fmt.Sprintf("build failed with %d error(s)", len(diags))
			}

			outputJSON(diagnostics.OutputEnvelope{
				Success:     false,
				Command:     "build",
				Error:       errMsg,
				Diagnostics: diags,
			})
			return nil
		}
		return fmt.Errorf("build task failed: %w", runErr)
	}

	// Locate artifact
	var searchPattern string
	if opts.AAB {
		searchPattern = filepath.Join(opts.TargetDir, "app", "build", "outputs", "bundle", "*", "*.aab")
	} else if opts.Release {
		searchPattern = filepath.Join(opts.TargetDir, "app", "build", "outputs", "apk", "release", "*.apk")
	} else {
		searchPattern = filepath.Join(opts.TargetDir, "app", "build", "outputs", "apk", "debug", "*.apk")
	}

	matches, _ := filepath.Glob(searchPattern)
	var artInfo *ArtifactInfo
	if len(matches) > 0 {
		artifactPath := matches[0]
		relPath, _ := filepath.Rel(opts.TargetDir, artifactPath)

		info, err := os.Stat(artifactPath)
		if err == nil {
			sizeMB := float64(info.Size()) / (1024 * 1024)
			hashStr := "N/A"
			f, err := os.Open(artifactPath)
			if err == nil {
				h := sha256.New()
				_, _ = io.Copy(h, f)
				f.Close()
				hashStr = hex.EncodeToString(h.Sum(nil))
			}

			artInfo = &ArtifactInfo{
				Type:      artifactType,
				File:      relPath,
				SizeBytes: info.Size(),
				SizeMB:    sizeMB,
				SHA256:    hashStr,
			}
		}
	}

	if opts.JSON {
		outputJSON(diagnostics.OutputEnvelope{
			Success: true,
			Command: "build",
			Data:    artInfo,
		})
		return nil
	}

	if artInfo != nil {
		fmt.Println("\n======================================================================")
		fmt.Printf("✓ %s BUILT SUCCESSFULLY!\n", strings.ToUpper(artifactType))
		fmt.Println("======================================================================")
		fmt.Printf("  File:    %s\n", artInfo.File)
		fmt.Printf("  Size:    %.2f MB (%d bytes)\n", artInfo.SizeMB, artInfo.SizeBytes)
		fmt.Printf("  SHA-256: %s\n", artInfo.SHA256)
		fmt.Println("======================================================================")
	} else {
		fmt.Println("✓ Build succeeded.")
	}

	return nil
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
