package builder

import "testing"

func TestResolveBuildTaskDefaultsToDebug(t *testing.T) {
	task, art := resolveBuildTask(BuildOptions{})
	if task != "assembleDebug" {
		t.Fatalf("default task = %q, want assembleDebug", task)
	}
	if art != "Debug APK" {
		t.Fatalf("default artifactType = %q, want Debug APK", art)
	}
}

func TestResolveBuildTaskReleaseAABModuleAndVariant(t *testing.T) {
	task, _ := resolveBuildTask(BuildOptions{Release: true})
	if task != "assembleRelease" {
		t.Errorf("release task = %q, want assembleRelease", task)
	}

	task, _ = resolveBuildTask(BuildOptions{AAB: true})
	if task != "bundleRelease" {
		t.Errorf("aab task = %q, want bundleRelease", task)
	}

	task, _ = resolveBuildTask(BuildOptions{Module: "app", Variant: "freeDebug"})
	if task != ":app:assembleFreeDebug" {
		t.Errorf("module+variant task = %q, want :app:assembleFreeDebug", task)
	}
}
