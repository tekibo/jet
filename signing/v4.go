package signing

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	apksignerSha256Re = regexp.MustCompile(`(?i)SHA-256 digest:\s*([0-9a-f:]+)`)
	keytoolSha256Re   = regexp.MustCompile(`(?i)SHA256:\s*([0-9a-f:]+)`)
)

// FindApksigner locates the latest apksigner executable in PATH or installed Android SDK build-tools.
func FindApksigner() (string, error) {
	if p, err := exec.LookPath("apksigner"); err == nil {
		return p, nil
	}

	var sdkRoots []string
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			sdkRoots = append(sdkRoots, v)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		sdkRoots = append(sdkRoots,
			filepath.Join(home, ".android", "sdk"),
			filepath.Join(home, "Android", "Sdk"),
		)
	}

	for _, root := range sdkRoots {
		btDir := filepath.Join(root, "build-tools")
		entries, err := os.ReadDir(btDir)
		if err != nil {
			continue
		}
		var versions []string
		for _, e := range entries {
			if e.IsDir() {
				versions = append(versions, e.Name())
			}
		}
		sort.Slice(versions, func(i, j int) bool {
			return compareVersionStrings(versions[i], versions[j]) > 0
		})
		for _, v := range versions {
			for _, name := range []string{"apksigner", "apksigner.bat"} {
				candidate := filepath.Join(btDir, v, name)
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					return candidate, nil
				}
			}
		}
	}

	return "", fmt.Errorf("apksigner executable not found in PATH or Android SDK build-tools")
}

func compareVersionStrings(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	maxLen := len(pa)
	if len(pb) > maxLen {
		maxLen = len(pb)
	}
	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(pa) {
			va, _ = strconv.Atoi(strings.SplitN(pa[i], "-", 2)[0])
		}
		if i < len(pb) {
			vb, _ = strconv.Atoi(strings.SplitN(pb[i], "-", 2)[0])
		}
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return strings.Compare(a, b)
}

func normalizeDigestHex(raw string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(raw), ":", ""))
}

func parseApksignerSHA256(output string) string {
	m := apksignerSha256Re.FindStringSubmatch(output)
	if len(m) < 2 {
		return ""
	}
	return normalizeDigestHex(m[1])
}

func parseKeytoolSHA256(output string) string {
	m := keytoolSha256Re.FindStringSubmatch(output)
	if len(m) < 2 {
		return ""
	}
	return normalizeDigestHex(m[1])
}

// EnsureDebugV4Signature ensures a matching APK Signature Scheme v4 (.idsig) companion file exists
// beside a standard debug-signed APK so Android 11+ adb installs can use Incremental Install.
// Custom/release-signed APKs whose certificate does not match the project's standard debug keystore
// are never modified.
func EnsureDebugV4Signature(targetDir, apkPath string) (bool, error) {
	apkInfo, err := os.Stat(apkPath)
	if err != nil {
		return false, err
	}

	idsigPath := apkPath + ".idsig"
	if idsigInfo, err := os.Stat(idsigPath); err == nil && !idsigInfo.ModTime().Before(apkInfo.ModTime()) {
		return true, nil
	}

	ksPath := FindStandardDebugKeystoreForProject(targetDir)
	if ksPath == "" {
		return false, nil
	}

	apksigner, err := FindApksigner()
	if err != nil {
		return false, nil
	}
	keytoolPath, err := findKeytool()
	if err != nil {
		return false, nil
	}

	verifyOut, err := exec.Command(apksigner, "verify", "--print-certs", apkPath).CombinedOutput()
	if err != nil {
		return false, nil
	}
	apkDigest := parseApksignerSHA256(string(verifyOut))
	if apkDigest == "" {
		return false, nil
	}

	ksOut, err := exec.Command(
		keytoolPath,
		"-list", "-v",
		"-keystore", ksPath,
		"-storepass", "android",
		"-alias", "androiddebugkey",
	).CombinedOutput()
	if err != nil {
		return false, nil
	}
	ksDigest := parseKeytoolSHA256(string(ksOut))
	if ksDigest == "" || apkDigest != ksDigest {
		return false, nil
	}

	signOut, err := exec.Command(
		apksigner,
		"sign",
		"--v1-signing-enabled", "false",
		"--v2-signing-enabled", "true",
		"--v3-signing-enabled", "false",
		"--v4-signing-enabled", "true",
		"--ks", ksPath,
		"--ks-pass", "pass:android",
		"--ks-key-alias", "androiddebugkey",
		apkPath,
	).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("apksigner v4 sign failed: %v (%s)", err, strings.TrimSpace(string(signOut)))
	}
	return true, nil
}
