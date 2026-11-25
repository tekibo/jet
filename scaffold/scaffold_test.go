package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestExecuteMapsCoreUIAndAppKotlinTemplatesAndSplashResources(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/project/gitignore.tmpl":                                                  {Data: []byte("/build\n")},
		"templates/project/app/src/main/kotlin/MainActivity.kt.tmpl":                        {Data: []byte("package {{.PackageName}}\n")},
		"templates/project/core-ui/src/main/kotlin/core/ui/Color.kt.tmpl":                   {Data: []byte("package {{.PackageName}}.core.ui\n")},
		"templates/project/core-ui/src/main/kotlin/core/ui/Type.kt.tmpl":                    {Data: []byte("package {{.PackageName}}.core.ui\n")},
		"templates/project/core-ui/src/main/kotlin/core/ui/Shape.kt.tmpl":                   {Data: []byte("package {{.PackageName}}.core.ui\n")},
		"templates/project/core-ui/src/main/kotlin/core/ui/Theme.kt.tmpl":                   {Data: []byte("package {{.PackageName}}.core.ui\n")},
		"templates/project/core-ui/src/main/kotlin/core/ui/Sheets.kt.tmpl":                  {Data: []byte("package {{.PackageName}}.core.ui\n")},
		"templates/project/core-ui/src/main/kotlin/core/ui/Primitives.kt.tmpl":              {Data: []byte("package {{.PackageName}}.core.ui\n")},
		"templates/project/core-ui/src/test/kotlin/core/ui/DynamicThemeSupportTest.kt.tmpl": {Data: []byte("package {{.PackageName}}.core.ui\n")},
		"templates/project/app/src/main/res/values/colors.xml":                              {Data: []byte("<resources/>")},
		"templates/project/app/src/main/res/values-night/colors.xml":                        {Data: []byte("<resources/>")},
		"templates/project/app/src/main/res/values-v33/themes.xml":                          {Data: []byte("<resources/>")},
		"templates/project/app/src/main/res/drawable/ic_launcher_monochrome.xml":            {Data: []byte("<vector/>")},
	}

	dest := t.TempDir()
	cfg := DefaultConfig()
	cfg.AppName = "DemoApp"
	cfg.PackageName = "com.demo.app"
	cfg.PackagePath = "com/demo/app"
	cfg.TargetDir = dest

	t.Setenv("ANDROID_AGENT_RULES_URL", "")
	if err := Execute(fsys, cfg); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := []string{
		".gitignore",
		"app/src/main/kotlin/com/demo/app/MainActivity.kt",
		"core-ui/src/main/kotlin/com/demo/app/core/ui/Color.kt",
		"core-ui/src/main/kotlin/com/demo/app/core/ui/Type.kt",
		"core-ui/src/main/kotlin/com/demo/app/core/ui/Shape.kt",
		"core-ui/src/main/kotlin/com/demo/app/core/ui/Theme.kt",
		"core-ui/src/main/kotlin/com/demo/app/core/ui/Sheets.kt",
		"core-ui/src/main/kotlin/com/demo/app/core/ui/Primitives.kt",
		"core-ui/src/test/kotlin/com/demo/app/core/ui/DynamicThemeSupportTest.kt",
		"app/src/main/res/values/colors.xml",
		"app/src/main/res/values-night/colors.xml",
		"app/src/main/res/values-v33/themes.xml",
		"app/src/main/res/drawable/ic_launcher_monochrome.xml",
	}
	for _, rel := range expected {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("missing scaffolded file %s: %v", rel, err)
		}
	}

	themeBytes, err := os.ReadFile(filepath.Join(dest, "core-ui/src/main/kotlin/com/demo/app/core/ui/Theme.kt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(themeBytes), "package com.demo.app.core.ui") {
		t.Fatalf("unexpected Theme.kt header: %q", string(themeBytes))
	}
}

func TestAddScreenUsesCoreUIIfPresent(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/screen/Screen.kt.tmpl": {Data: []byte("package {{.PackageName}}.ui.screens\n{{- if .HasCoreUI }}\nimport {{.PackageName}}.core.ui.AppSurfaceCard\nimport {{.PackageName}}.core.ui.AppTopBar\n{{- end }}\n")},
	}

	withCoreUI := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withCoreUI, "core-ui"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := AddScreen(fsys, withCoreUI, "Profile", "com.demo.app"); err != nil {
		t.Fatalf("AddScreen with core-ui failed: %v", err)
	}
	contentWith, err := os.ReadFile(filepath.Join(withCoreUI, "app/src/main/kotlin/com/demo/app/ui/screens/ProfileScreen.kt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contentWith), "import com.demo.app.core.ui.AppTopBar") || !strings.Contains(string(contentWith), "AppSurfaceCard") {
		t.Fatalf("expected core-ui imports in screen, got:\n%s", string(contentWith))
	}

	withoutCoreUI := t.TempDir()
	if err := AddScreen(fsys, withoutCoreUI, "ProfileScreen", "com.demo.app"); err != nil {
		t.Fatalf("AddScreen without core-ui failed: %v", err)
	}
	contentWithout, err := os.ReadFile(filepath.Join(withoutCoreUI, "app/src/main/kotlin/com/demo/app/ui/screens/ProfileScreen.kt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contentWithout), "core.ui") {
		t.Fatalf("did not expect core-ui imports without core-ui dir, got:\n%s", string(contentWithout))
	}
}
