package signing

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var debugStoreFileRegex = regexp.MustCompile(`(?i)storeFile\s*(?:=|\s)\s*(?:rootProject\.)?file\(\s*["']([^"']*debug\.keystore)["']\s*\)`)

func findKeytool() (string, error) {
	if p, err := exec.LookPath("keytool"); err == nil {
		return p, nil
	}
	for _, env := range []string{"JAVA_HOME", "JDK_HOME"} {
		if base := os.Getenv(env); base != "" {
			candidate := filepath.Join(base, "bin", "keytool")
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			if _, err := os.Stat(candidate + ".exe"); err == nil {
				return candidate + ".exe", nil
			}
		}
	}
	return "", fmt.Errorf("keytool executable not found in PATH or JAVA_HOME; ensure JDK 17+ is installed")
}

func candidateBuildScripts(targetDir string) []string {
	candidates := []string{
		filepath.Join(targetDir, "app", "build.gradle.kts"),
		filepath.Join(targetDir, "app", "build.gradle"),
		filepath.Join(targetDir, "build.gradle.kts"),
		filepath.Join(targetDir, "build.gradle"),
	}

	// Also check any first-level module build scripts in multi-module repos
	if entries, err := os.ReadDir(targetDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "app" || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			for _, name := range []string{"build.gradle.kts", "build.gradle"} {
				candidates = append(candidates, filepath.Join(targetDir, e.Name(), name))
			}
		}
	}
	return candidates
}

// FindStandardDebugKeystoreForProject returns an existing debug.keystore configured with
// standard Android debug credentials for targetDir, or falls back to the global debug keystore.
func FindStandardDebugKeystoreForProject(targetDir string) string {
	if targetDir == "" {
		targetDir = "."
	}
	for _, gradlePath := range candidateBuildScripts(targetDir) {
		data, err := os.ReadFile(gradlePath)
		if err != nil {
			continue
		}
		content := string(data)
		if !usesStandardDebugCredentials(content) {
			continue
		}
		matches := debugStoreFileRegex.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			destPath := resolveDebugKeystorePath(targetDir, filepath.Dir(gradlePath), m[0], m[1])
			if destPath != "" {
				if info, err := os.Stat(destPath); err == nil && !info.IsDir() {
					return destPath
				}
			}
		}
	}
	return findGlobalDebugKeystore()
}

// EnsureDebugKeystore checks if Gradle build files configure a local debug.keystore
// with standard Android debug credentials and generates it via keytool if missing.
// Returns the relative path of the generated keystore (or "" if none was needed).
func EnsureDebugKeystore(targetDir string, quiet bool) (string, error) {
	if targetDir == "" {
		targetDir = "."
	}

	candidates := candidateBuildScripts(targetDir)

	seenPaths := make(map[string]bool)
	for _, gradlePath := range candidates {
		data, err := os.ReadFile(gradlePath)
		if err != nil {
			continue
		}
		content := string(data)
		matches := debugStoreFileRegex.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			rawExpr := m[0]
			rawRef := m[1]
			destPath := resolveDebugKeystorePath(targetDir, filepath.Dir(gradlePath), rawExpr, rawRef)
			if destPath == "" || seenPaths[destPath] {
				continue
			}
			seenPaths[destPath] = true

			if _, err := os.Stat(destPath); err == nil {
				continue
			}

			// Do not auto-generate if custom non-standard credentials are specified.
			if !usesStandardDebugCredentials(content) {
				continue
			}

			reusedGlobal, err := provisionStandardDebugKeystore(destPath)
			if err != nil {
				return "", err
			}
			ensureGitignoreHas(targetDir, filepath.Base(destPath))

			rel, err := filepath.Rel(targetDir, destPath)
			if err != nil {
				rel = destPath
			}
			if !quiet {
				if reusedGlobal {
					fmt.Printf("[i] Missing debug keystore detected; copied global debug keystore -> '%s'\n", rel)
				} else {
					fmt.Printf("[i] Missing debug keystore detected; generated '%s'\n", rel)
				}
			}
			return rel, nil
		}
	}

	return "", nil
}

func resolveDebugKeystorePath(rootDir, moduleDir, fullMatch, rawRef string) string {
	lowerExpr := strings.ToLower(fullMatch)
	cleaned := strings.TrimSpace(rawRef)

	if strings.Contains(lowerExpr, "rootproject.file(") ||
		strings.Contains(cleaned, "${rootDir}") ||
		strings.Contains(cleaned, "$rootDir") ||
		strings.Contains(cleaned, "${project.rootDir}") ||
		strings.Contains(cleaned, "$rootProject.projectDir") {
		cleaned = strings.NewReplacer(
			"${rootDir}/", "",
			"${rootDir}\\", "",
			"$rootDir/", "",
			"$rootDir\\", "",
			"${project.rootDir}/", "",
			"$rootProject.projectDir/", "",
		).Replace(cleaned)
		cleaned = strings.TrimLeft(cleaned, "/\\")
		return filepath.Join(rootDir, filepath.FromSlash(cleaned))
	}

	if filepath.IsAbs(cleaned) {
		return cleaned
	}
	return filepath.Clean(filepath.Join(moduleDir, filepath.FromSlash(cleaned)))
}

func usesStandardDebugCredentials(content string) bool {
	// If storePassword or keyAlias are explicitly set near debug.keystore,
	// ensure we don't generate a keystore with mismatched credentials.
	passRe := regexp.MustCompile(`(?m)storePassword\s*(?:=|\s)\s*["']([^"']+)["']`)
	aliasRe := regexp.MustCompile(`(?m)keyAlias\s*(?:=|\s)\s*["']([^"']+)["']`)

	// Check lines around debug.keystore occurrences
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "debug.keystore") {
			continue
		}
		start := i - 2
		if start < 0 {
			start = 0
		}
		end := i + 6
		if end > len(lines) {
			end = len(lines)
		}
		window := strings.Join(lines[start:end], "\n")
		if pm := passRe.FindStringSubmatch(window); len(pm) > 1 && pm[1] != "android" {
			return false
		}
		if am := aliasRe.FindStringSubmatch(window); len(am) > 1 && am[1] != "androiddebugkey" {
			return false
		}
	}
	return true
}

func globalDebugKeystorePath() string {
	if v := strings.TrimSpace(os.Getenv("ANDROID_USER_HOME")); v != "" {
		return filepath.Join(v, "debug.keystore")
	}
	if v := strings.TrimSpace(os.Getenv("ANDROID_SDK_HOME")); v != "" {
		return filepath.Join(v, ".android", "debug.keystore")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".android", "debug.keystore")
	}
	return ""
}

func findGlobalDebugKeystore() string {
	if c := globalDebugKeystorePath(); c != "" {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func copyFile(src, dest string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dest, err)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// provisionStandardDebugKeystore populates destPath from the global Android debug keystore
// (~/.android/debug.keystore) when present, or creates that global keystore first and copies it
// so all local repos share a single consistent debug signing identity.
func provisionStandardDebugKeystore(destPath string) (reusedGlobal bool, err error) {
	if global := findGlobalDebugKeystore(); global != "" {
		if err := copyFile(global, destPath, 0600); err != nil {
			return false, fmt.Errorf("failed to copy global debug keystore %s: %w", global, err)
		}
		return true, nil
	}

	if globalDest := globalDebugKeystorePath(); globalDest != "" {
		if err := generateDebugKeystoreFile(globalDest); err == nil {
			if copyErr := copyFile(globalDest, destPath, 0600); copyErr == nil {
				return false, nil
			}
		}
	}

	if err := generateDebugKeystoreFile(destPath); err != nil {
		return false, err
	}
	return false, nil
}

func generateDebugKeystoreFile(destPath string) error {
	keytoolPath, err := findKeytool()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", destPath, err)
	}

	cmd := exec.Command(
		keytoolPath,
		"-genkeypair",
		"-keystore", destPath,
		"-alias", "androiddebugkey",
		"-keyalg", "RSA",
		"-keysize", "2048",
		"-validity", "10000",
		"-storepass", "android",
		"-keypass", "android",
		"-dname", "CN=Android Debug, O=Android, C=US",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to generate debug keystore at %s: %v (%s)", destPath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureGitignoreHas(targetDir string, entries ...string) {
	gitignorePath := filepath.Join(targetDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return
	}
	content := string(data)
	var additions []string
	for _, entry := range entries {
		if entry == "" || strings.Contains(content, entry) {
			continue
		}
		if strings.HasSuffix(entry, ".keystore") && strings.Contains(content, "*.keystore") {
			continue
		}
		if strings.HasSuffix(entry, ".jks") && strings.Contains(content, "*.jks") {
			continue
		}
		additions = append(additions, entry)
	}
	if len(additions) == 0 {
		return
	}
	prefix := ""
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		prefix = "\n"
	}
	_ = os.WriteFile(gitignorePath, []byte(content+prefix+strings.Join(additions, "\n")+"\n"), 0644)
}
