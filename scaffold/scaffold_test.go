package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestExecuteMapsThemeKotlinTemplatesAndSplashResources(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/project/gitignore.tmpl":                                       {Data: []byte("/build\n")},
		"templates/project/app/src/main/kotlin/MainActivity.kt.tmpl":             {Data: []byte("package {{.PackageName}}\n")},
		"templates/project/app/src/main/kotlin/ui/theme/Color.kt.tmpl":           {Data: []byte("package {{.PackageName}}.ui.theme\n")},
		"templates/project/app/src/main/kotlin/ui/theme/Type.kt.tmpl":            {Data: []byte("package {{.PackageName}}.ui.theme\n")},
		"templates/project/app/src/main/kotlin/ui/theme/Theme.kt.tmpl":           {Data: []byte("package {{.PackageName}}.ui.theme\n")},
		"templates/project/app/src/main/res/values/colors.xml":                   {Data: []byte("<resources/>")},
		"templates/project/app/src/main/res/values-night/colors.xml":             {Data: []byte("<resources/>")},
		"templates/project/app/src/main/res/values-v33/themes.xml":               {Data: []byte("<resources/>")},
		"templates/project/app/src/main/res/drawable/ic_launcher_monochrome.xml": {Data: []byte("<vector/>")},
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
		"app/src/main/kotlin/com/demo/app/ui/theme/Color.kt",
		"app/src/main/kotlin/com/demo/app/ui/theme/Type.kt",
		"app/src/main/kotlin/com/demo/app/ui/theme/Theme.kt",
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

	themeBytes, err := os.ReadFile(filepath.Join(dest, "app/src/main/kotlin/com/demo/app/ui/theme/Theme.kt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(themeBytes), "package com.demo.app.ui.theme") {
		t.Fatalf("unexpected Theme.kt header: %q", string(themeBytes))
	}
}
