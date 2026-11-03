package scaffold

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
)

type Config struct {
	AppName           string
	PackageName       string
	PackagePath       string
	TargetDir         string
	AGPVersion        string
	KotlinVersion     string
	ComposeBomVersion string
	MinSdk            int
	TargetSdk         int
	CompileSdk        int
	RemoteRulesURL    string
}

func DefaultConfig() Config {
	return Config{
		AppName:           "MyApp",
		PackageName:       "com.example.myapp",
		PackagePath:       "com/example/myapp",
		TargetDir:         ".",
		AGPVersion:        "9.1.1",
		KotlinVersion:     "2.2.0",
		ComposeBomVersion: "2025.02.00",
		MinSdk:            26,
		TargetSdk:         36,
		CompileSdk:        36,
	}
}

func PromptConfig(targetDir string) (Config, error) {
	cfg := DefaultConfig()
	if targetDir != "" {
		cfg.TargetDir = targetDir
	}

	reader := bufio.NewReader(os.Stdin)

	// Determine default app name from target directory
	baseDir := filepath.Base(cfg.TargetDir)
	if baseDir == "." || baseDir == "/" || baseDir == "" {
		curr, _ := os.Getwd()
		baseDir = filepath.Base(curr)
	}

	defaultAppName := toTitleCase(baseDir)
	if defaultAppName == "" {
		defaultAppName = "MyApp"
	}

	fmt.Printf("Project / App name [%s]: ", defaultAppName)
	appNameInput, _ := reader.ReadString('\n')
	appNameInput = strings.TrimSpace(appNameInput)
	if appNameInput != "" {
		cfg.AppName = appNameInput
	} else {
		cfg.AppName = defaultAppName
	}

	// Determine default package name
	cleanName := sanitizePkg(cfg.AppName)
	defaultPkg := fmt.Sprintf("com.example.%s", cleanName)

	pkgRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$`)

	for {
		fmt.Printf("Package name (e.g. com.mycompany.app) [%s]: ", defaultPkg)
		pkgInput, _ := reader.ReadString('\n')
		pkgInput = strings.TrimSpace(pkgInput)
		if pkgInput == "" {
			pkgInput = defaultPkg
		}

		if pkgRegex.MatchString(pkgInput) {
			cfg.PackageName = pkgInput
			break
		}
		fmt.Println(" [!] Invalid package name. Must be at least two segments (e.g., com.example.myapp). Try again.")
	}

	cfg.PackagePath = strings.ReplaceAll(cfg.PackageName, ".", "/")
	return cfg, nil
}

func Execute(templateFS fs.FS, cfg Config) error {
	fmt.Printf("\n[i] Initializing %s in '%s'...\n", cfg.AppName, cfg.TargetDir)

	if err := os.MkdirAll(cfg.TargetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// 1. Fetch remote dynamic AGENTS.md rules or manifest if configured/available
	var customAgentsMD string
	remoteURL := os.Getenv("ANDROID_AGENT_RULES_URL")
	if remoteURL != "" {
		fmt.Printf("[i] Fetching latest rules from %s...\n", remoteURL)
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(remoteURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			customAgentsMD = string(body)
			fmt.Println("    ✓ Loaded live remote AGENTS.md rules")
		} else {
			fmt.Println("    [!] Remote rules unreachable; using embedded offline guidelines.")
		}
	}

	// 2. Walk embedded templates and write project files
	err := fs.WalkDir(templateFS, "templates/project", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel("templates/project", path)
		if err != nil {
			return err
		}

		// Handle destination path mapping
		destRel := rel
		isTemplate := strings.HasSuffix(rel, ".tmpl")
		if isTemplate {
			destRel = strings.TrimSuffix(rel, ".tmpl")
		}

		// Special case: map gitignore.tmpl -> .gitignore
		if rel == "gitignore.tmpl" {
			destRel = ".gitignore"
		}

		// Map Kotlin source files to their full package directory
		if rel == "app/src/main/kotlin/MainActivity.kt.tmpl" {
			destRel = filepath.Join("app/src/main/kotlin", cfg.PackagePath, "MainActivity.kt")
		} else if rel == "app/src/main/kotlin/ui/theme/Theme.kt.tmpl" {
			destRel = filepath.Join("app/src/main/kotlin", cfg.PackagePath, "ui/theme", "Theme.kt")
		}

		destPath := filepath.Join(cfg.TargetDir, destRel)

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		data, err := fs.ReadFile(templateFS, path)
		if err != nil {
			return err
		}

		// Template execution vs raw binary write
		if isTemplate {
			var contentStr string
			if rel == "AGENTS.md.tmpl" && customAgentsMD != "" {
				contentStr = customAgentsMD
			} else {
				contentStr = string(data)
			}

			tmpl, err := template.New(rel).Parse(contentStr)
			if err != nil {
				return fmt.Errorf("error parsing template %s: %w", rel, err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, cfg); err != nil {
				return fmt.Errorf("error executing template %s: %w", rel, err)
			}

			if err := os.WriteFile(destPath, buf.Bytes(), 0644); err != nil {
				return err
			}
		} else {
			// Binary or static file (gradlew, gradle-wrapper.jar, etc.)
			mode := os.FileMode(0644)
			if filepath.Base(destPath) == "gradlew" {
				mode = 0755
			}
			if err := os.WriteFile(destPath, data, mode); err != nil {
				return err
			}
		}

		fmt.Printf("  ✓ %s\n", destRel)
		return nil
	})

	if err != nil {
		return err
	}

	// 3. Create symlink GEMINI.md -> AGENTS.md
	geminiLink := filepath.Join(cfg.TargetDir, "GEMINI.md")
	_ = os.Remove(geminiLink) // remove existing if any
	err = os.Symlink("AGENTS.md", geminiLink)
	if err != nil {
		// Fallback to copy if symlink is not supported (e.g. Windows without dev privileges)
		agentsPath := filepath.Join(cfg.TargetDir, "AGENTS.md")
		data, _ := os.ReadFile(agentsPath)
		_ = os.WriteFile(geminiLink, data, 0644)
		fmt.Println("  ✓ GEMINI.md (copied from AGENTS.md)")
	} else {
		fmt.Println("  ✓ GEMINI.md -> symlinked to AGENTS.md")
	}

	fmt.Printf("\n✓ Project %s successfully created in %s!\n\n", cfg.AppName, cfg.TargetDir)
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", cfg.TargetDir)
	fmt.Println("  ./gradlew assembleDebug    # Build APK")
	fmt.Println("  ./gradlew installDebug     # Install to connected device")

	return nil
}

func toTitleCase(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "")
}

func sanitizePkg(s string) string {
	var b strings.Builder
	for _, ch := range strings.ToLower(s) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
		}
	}
	res := b.String()
	if res == "" {
		res = "app"
	}
	return res
}

func AddScreen(templateFS fs.FS, targetDir, screenName, pkgName string) error {
	if screenName == "" {
		return fmt.Errorf("screen name cannot be empty")
	}
	if !strings.HasSuffix(screenName, "Screen") {
		screenName = screenName + "Screen"
	}

	pkgPath := strings.ReplaceAll(pkgName, ".", "/")
	destDir := filepath.Join(targetDir, "app", "src", "main", "kotlin", pkgPath, "ui", "screens")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	destPath := filepath.Join(destDir, screenName+".kt")
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("%s already exists", destPath)
	}

	tmplData, err := fs.ReadFile(templateFS, "templates/screen/Screen.kt.tmpl")
	if err != nil {
		return fmt.Errorf("could not read screen template: %w", err)
	}

	tmpl, err := template.New("screen").Parse(string(tmplData))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	data := struct {
		PackageName string
		ScreenName  string
	}{
		PackageName: pkgName,
		ScreenName:  screenName,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	if err := os.WriteFile(destPath, buf.Bytes(), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Created new Composable screen at: %s\n", destPath)
	return nil
}

func AddViewModel(templateFS fs.FS, targetDir, vmName, pkgName string) error {
	if vmName == "" {
		return fmt.Errorf("viewmodel name cannot be empty")
	}

	baseName := strings.TrimSuffix(vmName, "ViewModel")
	baseName = strings.TrimSuffix(baseName, "VM")
	if len(baseName) > 0 {
		baseName = strings.ToUpper(baseName[:1]) + baseName[1:]
	}

	pkgPath := strings.ReplaceAll(pkgName, ".", "/")
	destDir := filepath.Join(targetDir, "app", "src", "main", "kotlin", pkgPath, "ui", "viewmodels")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	destPath := filepath.Join(destDir, baseName+"ViewModel.kt")
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("%s already exists", destPath)
	}

	tmplData, err := fs.ReadFile(templateFS, "templates/viewmodel/ViewModel.kt.tmpl")
	if err != nil {
		return fmt.Errorf("could not read viewmodel template: %w", err)
	}

	tmpl, err := template.New("viewmodel").Parse(string(tmplData))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	data := struct {
		PackageName string
		BaseName    string
	}{
		PackageName: pkgName,
		BaseName:    baseName,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	if err := os.WriteFile(destPath, buf.Bytes(), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Created new ViewModel at: %s\n", destPath)
	return nil
}
