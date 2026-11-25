package scaffold

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	includeLineRegex   = regexp.MustCompile(`include\s*\(([^)]+)\)|include\s+([^\n]+)`)
	quotedModuleRegex  = regexp.MustCompile(`["'](:[^"']+)["']`)
	applicationIDRegex = regexp.MustCompile(`applicationId\s*(?:=|\s)\s*["']([^"']+)["']`)
	namespaceRegex     = regexp.MustCompile(`namespace\s*(?:=|\s)\s*["']([^"']+)["']`)
)

// FindModuleDirs returns all Gradle module directories in rootDir (excluding rootDir itself),
// discovered from settings.gradle(.kts) and immediate subdirectories with build scripts.
func FindModuleDirs(rootDir string) []string {
	if rootDir == "" {
		rootDir = "."
	}

	seen := make(map[string]bool)
	var dirs []string
	addDir := func(d string) {
		d = filepath.Clean(d)
		if seen[d] {
			return
		}
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}

	// 1. Parse settings.gradle.kts / settings.gradle
	for _, name := range []string{"settings.gradle.kts", "settings.gradle"} {
		data, err := os.ReadFile(filepath.Join(rootDir, name))
		if err != nil {
			continue
		}
		for _, lineMatch := range includeLineRegex.FindAllString(string(data), -1) {
			for _, m := range quotedModuleRegex.FindAllStringSubmatch(lineMatch, -1) {
				if len(m) < 2 {
					continue
				}
				rel := strings.TrimPrefix(m[1], ":")
				rel = strings.ReplaceAll(rel, ":", string(filepath.Separator))
				if rel != "" {
					addDir(filepath.Join(rootDir, rel))
				}
			}
		}
	}

	// 2. Scan immediate subdirectories containing build.gradle(.kts)
	if entries, err := os.ReadDir(rootDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "build" || e.Name() == "gradle" {
				continue
			}
			sub := filepath.Join(rootDir, e.Name())
			if hasGradleBuildFile(sub) {
				addDir(sub)
			}
		}
	}

	return dirs
}

func hasGradleBuildFile(dir string) bool {
	for _, f := range []string{"build.gradle.kts", "build.gradle"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}

// FindBuildArtifacts discovers built .apk or .aab artifacts across modules and flavors.
func FindBuildArtifacts(rootDir string, aab, release bool, moduleFilter string) []string {
	if rootDir == "" {
		rootDir = "."
	}

	ext := ".apk"
	kindDir := "apk"
	if aab {
		ext = ".aab"
		kindDir = "bundle"
	}

	wantVariant := "debug"
	if release || aab {
		wantVariant = "release"
	}

	var searchRoots []string
	if moduleFilter != "" {
		mod := strings.TrimPrefix(moduleFilter, ":")
		mod = strings.ReplaceAll(mod, ":", string(filepath.Separator))
		searchRoots = []string{filepath.Join(rootDir, mod)}
	} else {
		// Prioritize app module first, then any other module
		appDir := filepath.Join(rootDir, "app")
		if info, err := os.Stat(appDir); err == nil && info.IsDir() {
			searchRoots = append(searchRoots, appDir)
		}
		for _, d := range FindModuleDirs(rootDir) {
			if filepath.Clean(d) != filepath.Clean(appDir) {
				searchRoots = append(searchRoots, d)
			}
		}
	}

	var signedOrMatching []string
	var unsignedFallback []string

	for _, modDir := range searchRoots {
		outputsRoot := filepath.Join(modDir, "build", "outputs", kindDir)
		if _, err := os.Stat(outputsRoot); err != nil {
			continue
		}
		_ = filepath.WalkDir(outputsRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(d.Name()), ext) {
				return nil
			}
			lowerPath := strings.ToLower(filepath.ToSlash(path))
			if strings.Contains(lowerPath, "/androidtest/") || strings.Contains(d.Name(), "-androidTest") {
				return nil
			}
			// Match build type directory when present
			if !strings.Contains(lowerPath, "/"+wantVariant+"/") && !strings.Contains(strings.ToLower(d.Name()), wantVariant) {
				return nil
			}
			if strings.Contains(strings.ToLower(d.Name()), "-unsigned") {
				unsignedFallback = append(unsignedFallback, path)
			} else {
				signedOrMatching = append(signedOrMatching, path)
			}
			return nil
		})
	}

	if len(signedOrMatching) > 0 {
		sort.Strings(signedOrMatching)
		return signedOrMatching
	}
	sort.Strings(unsignedFallback)
	return unsignedFallback
}

// FindTestResultDirs finds all test-results directories containing TEST-*.xml across all modules.
func FindTestResultDirs(rootDir string) []string {
	if rootDir == "" {
		rootDir = "."
	}

	candidates := append([]string{rootDir}, FindModuleDirs(rootDir)...)
	seen := make(map[string]bool)
	var resultDirs []string

	for _, modDir := range candidates {
		trRoot := filepath.Join(modDir, "build", "test-results")
		entries, err := os.ReadDir(trRoot)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dirPath := filepath.Join(trRoot, e.Name())
			if seen[dirPath] || !containsTestXML(dirPath) {
				continue
			}
			seen[dirPath] = true
			resultDirs = append(resultDirs, dirPath)
		}
	}
	sort.Strings(resultDirs)
	return resultDirs
}

func containsTestXML(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "TEST-") && strings.HasSuffix(e.Name(), ".xml") {
			return true
		}
	}
	return false
}

// DetectAppLaunchTarget finds the Android applicationId and launcher activity component
// (e.g. "com.example.app", ".MainActivity") across merged or source manifests and build scripts.
func DetectAppLaunchTarget(rootDir string) (string, string, error) {
	if rootDir == "" {
		rootDir = "."
	}

	appDirs := findApplicationModuleDirs(rootDir)

	// 1. Try merged manifests first (exact applicationId + merged launcher activity)
	for _, modDir := range appDirs {
		mergedRoot := filepath.Join(modDir, "build", "intermediates")
		var mergedManifests []string
		for _, sub := range []string{"merged_manifest", "merged_manifests"} {
			p := filepath.Join(mergedRoot, sub)
			_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() && d.Name() == "AndroidManifest.xml" {
					mergedManifests = append(mergedManifests, path)
				}
				return nil
			})
		}
		for _, mf := range mergedManifests {
			pkg, act := parseManifestLaunchTarget(mf)
			if pkg != "" {
				if act == "" {
					act = ".MainActivity"
				}
				return pkg, act, nil
			}
		}
	}

	// 2. Fall back to build.gradle(.kts) + source AndroidManifest.xml
	for _, modDir := range appDirs {
		pkg := detectPackageFromGradle(modDir)
		srcManifest := filepath.Join(modDir, "src", "main", "AndroidManifest.xml")
		mPkg, act := parseManifestLaunchTarget(srcManifest)
		if pkg == "" {
			pkg = mPkg
		}
		if pkg != "" {
			if act == "" {
				act = ".MainActivity"
			}
			return pkg, act, nil
		}
	}

	return "", "", fmt.Errorf("could not detect Android application package in %s", rootDir)
}

func findApplicationModuleDirs(rootDir string) []string {
	var appDirs []string
	var otherDirs []string

	candidates := FindModuleDirs(rootDir)
	for _, d := range candidates {
		isApp := filepath.Base(d) == "app"
		for _, f := range []string{"build.gradle.kts", "build.gradle"} {
			if data, err := os.ReadFile(filepath.Join(d, f)); err == nil {
				s := string(data)
				if strings.Contains(s, "android.application") || strings.Contains(s, "com.android.application") {
					isApp = true
					break
				}
			}
		}
		if isApp {
			appDirs = append(appDirs, d)
		} else {
			otherDirs = append(otherDirs, d)
		}
	}

	return append(appDirs, otherDirs...)
}

func detectPackageFromGradle(modDir string) string {
	for _, f := range []string{"build.gradle.kts", "build.gradle"} {
		data, err := os.ReadFile(filepath.Join(modDir, f))
		if err != nil {
			continue
		}
		s := string(data)
		if m := applicationIDRegex.FindStringSubmatch(s); len(m) > 1 {
			return m[1]
		}
		if m := namespaceRegex.FindStringSubmatch(s); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

type xmlManifest struct {
	Package     string         `xml:"package,attr"`
	Application xmlApplication `xml:"application"`
}

type xmlApplication struct {
	Activities []xmlActivity `xml:"activity"`
	Aliases    []xmlActivity `xml:"activity-alias"`
}

type xmlActivity struct {
	Name          string            `xml:"http://schemas.android.com/apk/res/android name,attr"`
	NamePlain     string            `xml:"name,attr"`
	IntentFilters []xmlIntentFilter `xml:"intent-filter"`
}

type xmlIntentFilter struct {
	Actions    []xmlNamed `xml:"action"`
	Categories []xmlNamed `xml:"category"`
}

type xmlNamed struct {
	Name      string `xml:"http://schemas.android.com/apk/res/android name,attr"`
	NamePlain string `xml:"name,attr"`
}

func parseManifestLaunchTarget(manifestPath string) (string, string) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", ""
	}
	var m xmlManifest
	if err := xml.Unmarshal(data, &m); err != nil {
		return "", ""
	}

	for _, act := range append(m.Application.Activities, m.Application.Aliases...) {
		name := act.Name
		if name == "" {
			name = act.NamePlain
		}
		if name == "" {
			continue
		}
		for _, f := range act.IntentFilters {
			hasMain := false
			hasLauncher := false
			for _, a := range f.Actions {
				n := a.Name
				if n == "" {
					n = a.NamePlain
				}
				if n == "android.intent.action.MAIN" {
					hasMain = true
				}
			}
			for _, c := range f.Categories {
				n := c.Name
				if n == "" {
					n = c.NamePlain
				}
				if n == "android.intent.category.LAUNCHER" {
					hasLauncher = true
				}
			}
			if hasMain && hasLauncher {
				return m.Package, name
			}
		}
	}

	return m.Package, ""
}
