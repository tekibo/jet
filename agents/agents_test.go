package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func skillLinkOK(t *testing.T, home string) {
	t.Helper()
	for _, d := range []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
	} {
		p := filepath.Join(d, skillName)
		if linkTarget(p) != skillDir() {
			t.Errorf("%s: link target is %q, want the checkout's skill %q", p, linkTarget(p), skillDir())
		}
	}
}

func skillLinksGone(t *testing.T, home string) {
	t.Helper()
	for _, d := range []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
	} {
		p := filepath.Join(d, skillName)
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("%s: still there after Uninstall", p)
		}
	}
}

func TestInstallAndUninstall(t *testing.T) {
	home := setHome(t)

	if err := Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if target := linkTarget(filepath.Join(home, ".local", "bin", "jet")); target != jetBin() {
		t.Errorf("~/.local/bin/jet -> %q, want the checkout's binary %q", target, jetBin())
	}
	skillLinkOK(t, home)

	claudePath := filepath.Join(home, ".claude", "CLAUDE.md")
	claude := readFile(t, claudePath)
	if !strings.Contains(claude, beginMarker) || !strings.Contains(claude, endMarker) {
		t.Error("~/.claude/CLAUDE.md: Jet section not between the markers")
	}
	if !strings.Contains(claude, "jet doctor") {
		t.Error("~/.claude/CLAUDE.md: the Jet section doesn't mention jet doctor")
	}

	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".local", "bin", "jet")); err == nil {
		t.Error("~/.local/bin/jet still there after Uninstall")
	}
	skillLinksGone(t, home)
	// Uninstall leaves the instructions in place.
	if !strings.Contains(readFile(t, claudePath), beginMarker) {
		t.Error("Uninstall removed the Jet instructions it leaves behind")
	}
}

func TestInstallTwiceIsIdempotent(t *testing.T) {
	home := setHome(t)

	for i := 0; i < 2; i++ {
		if err := Install(); err != nil {
			t.Fatalf("Install run %d: %v", i+1, err)
		}
	}
	if target := linkTarget(filepath.Join(home, ".local", "bin", "jet")); target != jetBin() {
		t.Errorf("~/.local/bin/jet -> %q, want the checkout's binary %q", target, jetBin())
	}
	skillLinkOK(t, home)
	claude := readFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	if strings.Count(claude, beginMarker) != 1 {
		t.Errorf("Jet section appears %d times in ~/.claude/CLAUDE.md, want 1", strings.Count(claude, beginMarker))
	}
}

func TestInstallReplacesExistingSnippet(t *testing.T) {
	setHome(t)
	claudePath := filepath.Join(os.Getenv("HOME"), ".claude", "CLAUDE.md")
	writeFile(t, claudePath, beginMarker+"\nOLD SECTION\n"+endMarker+"\nrest of the file\n")

	if err := Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	claude := readFile(t, claudePath)
	if strings.Contains(claude, "OLD SECTION") {
		t.Error("old Jet section still in ~/.claude/CLAUDE.md")
	}
	if strings.Count(claude, beginMarker) != 1 || strings.Count(claude, endMarker) != 1 {
		t.Errorf("Jet section not cleanly between the markers:\n%s", claude)
	}
	if !strings.Contains(claude, "rest of the file") {
		t.Error("Install clobbered the rest of ~/.claude/CLAUDE.md")
	}
}

func TestInstallAppendsToExistingFile(t *testing.T) {
	setHome(t)
	claudePath := filepath.Join(os.Getenv("HOME"), ".claude", "CLAUDE.md")
	writeFile(t, claudePath, "existing text\n")

	if err := Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	claude := readFile(t, claudePath)
	if !strings.HasPrefix(claude, "existing text") {
		t.Errorf("~/.claude/CLAUDE.md doesn't start with the existing text:\n%s", claude)
	}
	if !strings.Contains(claude, beginMarker) || !strings.Contains(claude, endMarker) {
		t.Error("Jet section not appended")
	}
}

func TestInstallSkipsMissingInstructionDirs(t *testing.T) {
	setHome(t)
	// ~/.codex doesn't exist in the test home: only ~/.claude/CLAUDE.md is wired.
	if err := Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".codex")); !os.IsNotExist(err) {
		t.Error("~/.codex shouldn't be created by Install")
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".claude", "CLAUDE.md")); err != nil {
		t.Error("~/.claude/CLAUDE.md should be created by Install")
	}
}
