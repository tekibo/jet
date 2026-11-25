package signing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDebugKeystoreGeneratesWhenConfiguredAndMissing(t *testing.T) {
	if _, err := findKeytool(); err != nil {
		t.Skipf("keytool not available: %v", err)
	}

	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	gradleKts := `
android {
  signingConfigs {
    create("debugConfig") {
      storeFile = file("${rootDir}/debug.keystore")
      storePassword = "android"
      keyAlias = "androiddebugkey"
      keyPassword = "android"
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(gradleKts), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.iml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	rel, err := EnsureDebugKeystore(dir, true)
	if err != nil {
		t.Fatalf("EnsureDebugKeystore failed: %v", err)
	}
	if rel != "debug.keystore" {
		t.Fatalf("got rel=%q, want debug.keystore", rel)
	}
	if _, err := os.Stat(filepath.Join(dir, "debug.keystore")); err != nil {
		t.Fatalf("expected debug.keystore to exist: %v", err)
	}

	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gi), "debug.keystore") {
		t.Errorf("expected .gitignore to contain debug.keystore, got:\n%s", string(gi))
	}

	// Second call is a no-op
	rel2, err := EnsureDebugKeystore(dir, true)
	if err != nil {
		t.Fatalf("second EnsureDebugKeystore failed: %v", err)
	}
	if rel2 != "" {
		t.Errorf("expected empty rel on second call, got %q", rel2)
	}
}

func TestEnsureDebugKeystoreSkipsCustomPassword(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	gradleKts := `
android {
  signingConfigs {
    create("debugConfig") {
      storeFile = file("${rootDir}/debug.keystore")
      storePassword = "custom-secret-password"
      keyAlias = "my-custom-alias"
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(gradleKts), 0644); err != nil {
		t.Fatal(err)
	}

	rel, err := EnsureDebugKeystore(dir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != "" {
		t.Errorf("expected no keystore generated for custom credentials, got %q", rel)
	}
	if _, err := os.Stat(filepath.Join(dir, "debug.keystore")); !os.IsNotExist(err) {
		t.Errorf("debug.keystore should not have been generated for custom credentials")
	}
}
