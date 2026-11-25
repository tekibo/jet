package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindModuleDirsAndTestResultDirs(t *testing.T) {
	root := t.TempDir()
	settings := `
rootProject.name = "Multi"
include(":core-model")
include(":feature:editor", ":app")
`
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}

	for _, mod := range []string{"core-model", "feature/editor", "app"} {
		dir := filepath.Join(root, mod)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	mods := FindModuleDirs(root)
	if len(mods) != 3 {
		t.Fatalf("expected 3 modules, got %d: %v", len(mods), mods)
	}

	// Add test-results in core-model and app
	for _, mod := range []string{"core-model", "app"} {
		tr := filepath.Join(root, mod, "build", "test-results", "testDebugUnitTest")
		if err := os.MkdirAll(tr, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tr, "TEST-Foo.xml"), []byte("<testsuite/>"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	trDirs := FindTestResultDirs(root)
	if len(trDirs) != 2 {
		t.Fatalf("expected 2 test result dirs, got %d: %v", len(trDirs), trDirs)
	}
}

func TestFindBuildArtifactsPrefersSignedAndFlavor(t *testing.T) {
	root := t.TempDir()
	apkDir := filepath.Join(root, "app", "build", "outputs", "apk", "free", "debug")
	if err := os.MkdirAll(apkDir, 0755); err != nil {
		t.Fatal(err)
	}
	signed := filepath.Join(apkDir, "app-free-debug.apk")
	unsigned := filepath.Join(apkDir, "app-free-debug-unsigned.apk")
	if err := os.WriteFile(signed, []byte("apk"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unsigned, []byte("apk"), 0644); err != nil {
		t.Fatal(err)
	}

	artifacts := FindBuildArtifacts(root, false, false, "")
	if len(artifacts) != 1 || artifacts[0] != signed {
		t.Fatalf("got artifacts %v, want [%s]", artifacts, signed)
	}
}

func TestDetectAppLaunchTargetCustomActivity(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	manifestDir := filepath.Join(appDir, "src", "main")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	gradle := `
plugins { id("com.android.application") }
android {
  namespace = "com.example.ns"
  defaultConfig { applicationId = "com.example.real" }
}
`
	if err := os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(gradle), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
  <application>
    <activity android:name=".ui.LauncherActivity">
      <intent-filter>
        <action android:name="android.intent.action.MAIN" />
        <category android:name="android.intent.category.LAUNCHER" />
      </intent-filter>
    </activity>
  </application>
</manifest>`
	if err := os.WriteFile(filepath.Join(manifestDir, "AndroidManifest.xml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	pkg, act, err := DetectAppLaunchTarget(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg != "com.example.real" {
		t.Errorf("pkg = %q, want com.example.real", pkg)
	}
	if act != ".ui.LauncherActivity" {
		t.Errorf("act = %q, want .ui.LauncherActivity", act)
	}
}
