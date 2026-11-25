package pkgchanger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRenameMovesMultiModuleSourcesAndUpdatesReferences(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"settings.gradle.kts":                                                    "include(\":core-ui\")\ninclude(\":app\")\n",
		"app/build.gradle.kts":                                                   "android {\n\tnamespace = \"com.old.app\"\n\tdefaultConfig {\n\t\tapplicationId = \"com.old.app\"\n\t}\n}\n",
		"core-ui/build.gradle.kts":                                               "android {\n\tnamespace = \"com.old.app.core.ui\"\n}\n",
		"app/src/main/kotlin/com/old/app/MainActivity.kt":                        "package com.old.app\n\nimport com.old.app.core.ui.AppTheme\n",
		"core-ui/src/main/kotlin/com/old/app/core/ui/Theme.kt":                   "package com.old.app.core.ui\n",
		"core-ui/src/test/kotlin/com/old/app/core/ui/DynamicThemeSupportTest.kt": "package com.old.app.core.ui\n",
	}
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	err := RunRename(RenameOptions{
		TargetDir:  dir,
		OldPackage: "com.old.app",
		NewPackage: "org.new.brand",
	})
	if err != nil {
		t.Fatalf("RunRename failed: %v", err)
	}

	expectedMoved := []string{
		"app/src/main/kotlin/org/new/brand/MainActivity.kt",
		"core-ui/src/main/kotlin/org/new/brand/core/ui/Theme.kt",
		"core-ui/src/test/kotlin/org/new/brand/core/ui/DynamicThemeSupportTest.kt",
	}
	for _, rel := range expectedMoved {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected moved file %s: %v", rel, err)
		}
	}

	oldGone := []string{
		"app/src/main/kotlin/com",
		"core-ui/src/main/kotlin/com",
		"core-ui/src/test/kotlin/com",
	}
	for _, rel := range oldGone {
		if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
			t.Errorf("expected empty old directory %s to be removed", rel)
		}
	}

	coreGradle, err := os.ReadFile(filepath.Join(dir, "core-ui/build.gradle.kts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(coreGradle), `namespace = "org.new.brand.core.ui"`) {
		t.Fatalf("expected updated core-ui namespace, got:\n%s", string(coreGradle))
	}

	mainAct, err := os.ReadFile(filepath.Join(dir, "app/src/main/kotlin/org/new/brand/MainActivity.kt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainAct), "import org.new.brand.core.ui.AppTheme") {
		t.Fatalf("expected updated core.ui import in MainActivity, got:\n%s", string(mainAct))
	}
}
