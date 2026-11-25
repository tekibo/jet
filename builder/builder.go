package builder

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"jet/diagnostics"
	"jet/scaffold"
	"jet/signing"
)

type BuildOptions struct {
	TargetDir string
	Release   bool
	AAB       bool
	JSON      bool
	Module    string
	Variant   string
	Templates fs.FS
}

type ArtifactInfo struct {
	Type           string         `json:"type"`
	File           string         `json:"file"`
	SizeBytes      int64          `json:"size_bytes"`
	SizeMB         float64        `json:"size_mb"`
	SHA256         string         `json:"sha256"`
	ExtraArtifacts []ArtifactInfo `json:"extra_artifacts,omitempty"`
}

func BuildApp(opts BuildOptions) error {
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}
	start := time.Now()

	failJSON := func(err error) error {
		if opts.JSON {
			outputJSON(diagnostics.OutputEnvelope{
				Success:    false,
				Command:    "build",
				Error:      err.Error(),
				DurationMs: time.Since(start).Milliseconds(),
			})
			return diagnostics.ExitError{Code: 1, Err: err}
		}
		return err
	}

	if err := scaffold.EnsureGradleWrapper(opts.Templates, opts.TargetDir, opts.JSON); err != nil {
		return failJSON(err)
	}

	if !opts.Release && !opts.AAB {
		if _, err := signing.EnsureDebugKeystore(opts.TargetDir, opts.JSON); err != nil {
			return failJSON(err)
		}
	}

	task, artifactType := resolveBuildTask(opts)

	if !opts.JSON {
		fmt.Printf("[i] Building %s via task '%s'...\n", artifactType, task)
	}

	cmd := exec.Command("./gradlew", task, "--console=plain")
	cmd.Dir = opts.TargetDir

	var outBuf bytes.Buffer
	if opts.JSON {
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	runErr := cmd.Run()
	elapsedMs := time.Since(start).Milliseconds()

	if runErr != nil {
		if opts.JSON {
			parsed := diagnostics.ParseGradleResult(outBuf.String())
			errMsg := parsed.SummaryError
			if errMsg == "" || errMsg == "build failed" {
				if len(parsed.FailedTasks) > 0 {
					errMsg = fmt.Sprintf("build failed at task %s", strings.Join(parsed.FailedTasks, ", "))
				} else {
					errMsg = fmt.Sprintf("build task failed: %v", runErr)
				}
			}

			outputJSON(diagnostics.OutputEnvelope{
				Success:       false,
				Command:       "build",
				Error:         errMsg,
				FailedTasks:   parsed.FailedTasks,
				DurationMs:    elapsedMs,
				Diagnostics:   parsed.Diagnostics,
				RawOutputTail: parsed.RawOutputTail,
			})
			return diagnostics.ExitError{Code: 1, Err: runErr}
		}
		return fmt.Errorf("build task failed: %w", runErr)
	}

	matches := scaffold.FindBuildArtifacts(opts.TargetDir, opts.AAB, opts.Release, opts.Module)
	var artInfo *ArtifactInfo
	if len(matches) > 0 {
		primary := buildArtifactInfo(opts.TargetDir, artifactType, matches[0])
		if len(matches) > 1 {
			for _, extraPath := range matches[1:] {
				extra := buildArtifactInfo(opts.TargetDir, artifactType, extraPath)
				if extra != nil {
					primary.ExtraArtifacts = append(primary.ExtraArtifacts, *extra)
				}
			}
		}
		artInfo = primary
	}

	if opts.JSON {
		outputJSON(diagnostics.OutputEnvelope{
			Success:    true,
			Command:    "build",
			DurationMs: elapsedMs,
			Data:       artInfo,
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
		for _, extra := range artInfo.ExtraArtifacts {
			fmt.Printf("  Extra:   %s (%.2f MB)\n", extra.File, extra.SizeMB)
		}
		fmt.Println("======================================================================")
	} else {
		fmt.Println("✓ Build succeeded.")
	}

	return nil
}

func resolveBuildTask(opts BuildOptions) (string, string) {
	prefix := "assemble"
	artifactType := "Debug APK"
	variant := "Debug"

	if opts.AAB {
		prefix = "bundle"
		artifactType = "Release Android App Bundle (.aab)"
		variant = "Release"
	} else if opts.Release {
		prefix = "assemble"
		artifactType = "Release APK"
		variant = "Release"
	}

	if opts.Variant != "" {
		variant = capitalizeFirst(opts.Variant)
		if opts.AAB {
			artifactType = fmt.Sprintf("%s Android App Bundle (.aab)", variant)
		} else {
			artifactType = fmt.Sprintf("%s APK", variant)
		}
	}

	task := prefix + variant
	if opts.Module != "" {
		mod := strings.TrimSpace(opts.Module)
		if !strings.HasPrefix(mod, ":") {
			mod = ":" + mod
		}
		task = mod + ":" + task
	}
	return task, artifactType
}

func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func buildArtifactInfo(targetDir, artifactType, artifactPath string) *ArtifactInfo {
	relPath, err := filepath.Rel(targetDir, artifactPath)
	if err != nil {
		relPath = artifactPath
	}

	info, err := os.Stat(artifactPath)
	if err != nil {
		return nil
	}

	sizeMB := float64(info.Size()) / (1024 * 1024)
	hashStr := "N/A"
	if f, err := os.Open(artifactPath); err == nil {
		h := sha256.New()
		_, _ = io.Copy(h, f)
		f.Close()
		hashStr = hex.EncodeToString(h.Sum(nil))
	}

	return &ArtifactInfo{
		Type:      artifactType,
		File:      relPath,
		SizeBytes: info.Size(),
		SizeMB:    sizeMB,
		SHA256:    hashStr,
	}
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
