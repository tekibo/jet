package signing

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDebugKeystoreGeneratesWhenConfiguredAndMissing(t *testing.T) {
	if _, err := findKeytool(); err != nil {
		t.Skipf("keytool not available: %v", err)
	}
	t.Setenv("ANDROID_USER_HOME", t.TempDir())
	t.Setenv("ANDROID_SDK_HOME", "")

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

func TestEnsureDebugKeystoreSeedsAndReusesGlobalAcrossRepos(t *testing.T) {
	if _, err := findKeytool(); err != nil {
		t.Skipf("keytool not available: %v", err)
	}

	androidUserHome := t.TempDir()
	t.Setenv("ANDROID_USER_HOME", androidUserHome)
	t.Setenv("ANDROID_SDK_HOME", "")

	writeRepo := func() string {
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
    }
  }
}
`
		if err := os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(gradleKts), 0644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	repo1 := writeRepo()
	if _, err := EnsureDebugKeystore(repo1, true); err != nil {
		t.Fatalf("EnsureDebugKeystore repo1 failed: %v", err)
	}

	globalBytes, err := os.ReadFile(filepath.Join(androidUserHome, "debug.keystore"))
	if err != nil {
		t.Fatalf("expected global debug.keystore to be seeded: %v", err)
	}
	repo1Bytes, err := os.ReadFile(filepath.Join(repo1, "debug.keystore"))
	if err != nil {
		t.Fatalf("expected repo1 debug.keystore: %v", err)
	}
	if !bytes.Equal(repo1Bytes, globalBytes) {
		t.Fatalf("expected repo1 debug.keystore to match seeded global debug.keystore")
	}

	repo2 := writeRepo()
	if _, err := EnsureDebugKeystore(repo2, true); err != nil {
		t.Fatalf("EnsureDebugKeystore repo2 failed: %v", err)
	}
	repo2Bytes, err := os.ReadFile(filepath.Join(repo2, "debug.keystore"))
	if err != nil {
		t.Fatalf("expected repo2 debug.keystore: %v", err)
	}
	if !bytes.Equal(repo2Bytes, globalBytes) {
		t.Fatalf("expected repo2 debug.keystore to reuse identical global debug.keystore bytes")
	}

	if found := FindStandardDebugKeystoreForProject(repo1); found != filepath.Join(repo1, "debug.keystore") {
		t.Fatalf("FindStandardDebugKeystoreForProject = %q, want %q", found, filepath.Join(repo1, "debug.keystore"))
	}
}

func TestParseDigestHelpers(t *testing.T) {
	apkOut := "Signer #1 certificate SHA-256 digest: 221049a62c87caffab9645d9efcf8da4c0ea69bf369f72ec5aacfcc337d2205e\n"
	ksOut := "\t SHA256: 22:10:49:A6:2C:87:CA:FF:AB:96:45:D9:EF:CF:8D:A4:C0:EA:69:BF:36:9F:72:EC:5A:AC:FC:C3:37:D2:20:5E\n"

	gotApk := parseApksignerSHA256(apkOut)
	gotKs := parseKeytoolSHA256(ksOut)
	if gotApk == "" || gotApk != gotKs {
		t.Fatalf("expected matching normalized digests, got apk=%q ks=%q", gotApk, gotKs)
	}
}
