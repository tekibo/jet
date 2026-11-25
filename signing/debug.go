package signing

import (
	"fmt"
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

// EnsureDebugKeystore checks if Gradle build files configure a local debug.keystore
// with standard Android debug credentials and generates it via keytool if missing.
// Returns the relative path of the generated keystore (or "" if none was needed).
func EnsureDebugKeystore(targetDir string, quiet bool) (string, error) {
	if targetDir == "" {
		targetDir = "."
	}

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

			if err := generateDebugKeystoreFile(destPath); err != nil {
				return "", err
			}
			ensureGitignoreHas(targetDir, filepath.Base(destPath))

			rel, err := filepath.Rel(targetDir, destPath)
			if err != nil {
				rel = destPath
			}
			if !quiet {
				fmt.Printf("[i] Missing debug keystore detected; generated '%s'\n", rel)
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
