// Package agents installs and uninstalls jet's agent integration: the jet
// CLI, the android-jet skill, and the Jet instructions in the agents' global
// instruction files.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// repoRoot is the jet checkout this package was built from.
var repoRoot = func() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	p, err := filepath.Abs(filepath.Dir(filepath.Dir(file)))
	if err != nil {
		return "."
	}
	return p
}()

const (
	skillName   = "android-jet"
	beginMarker = "<!-- jet:agents begin -->"
	endMarker   = "<!-- jet:agents end -->"
)

// jetBin is the jet binary the checkout builds into its root.
func jetBin() string { return filepath.Join(repoRoot, "jet") }

// skillDir is the android-jet skill directory in the checkout.
func skillDir() string { return filepath.Join(repoRoot, "skills", skillName) }

// snippetPath is the Jet instructions file for the agents.
func snippetPath() string { return filepath.Join(repoRoot, "src", "agents", "jet-snippet.md") }

// agentInstructionFiles returns the agents' global instruction files that
// exist here; files in missing directories are left alone.
func agentInstructionFiles(home string) []string {
	var files []string
	for _, f := range []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		if _, err := os.Stat(filepath.Dir(f)); err != nil {
			continue
		}
		files = append(files, f)
	}
	return files
}

// linkForce links linkPath to target, replacing whatever is there.
func linkForce(target, linkPath string) error {
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(target, linkPath)
}

// linkIfFree links linkPath to target unless an entry already lives there
// that jet didn't put there: warn and leave it.
func linkIfFree(target, linkPath string) error {
	if _, err := os.Lstat(linkPath); err == nil {
		if _, err := os.Readlink(linkPath); err != nil {
			fmt.Printf("  warn: %s exists and isn't jet's link; left as is\n", linkPath)
			return nil
		}
		if err := os.Remove(linkPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(target, linkPath)
}

// linkTarget returns the target of the symlink at path, or "" when path
// isn't a symlink.
func linkTarget(path string) string {
	t, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return t
}

// Install wires jet into the coding agents: the jet CLI into ~/.local/bin,
// the android-jet skill into ~/.claude/skills and ~/.agents/skills, and the
// Jet instructions between the jet markers in the agents' instruction files
// (each gets a backup as .bak-jet before it changes).
func Install() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(jetBin()); err != nil {
		return fmt.Errorf("jet binary not found in the checkout: %s", jetBin())
	}

	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	if err := linkForce(jetBin(), filepath.Join(binDir, "jet")); err != nil {
		return err
	}
	fmt.Printf("linked: jet -> %s\n", filepath.Join(binDir, "jet"))

	for _, d := range []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		if err := linkIfFree(skillDir(), filepath.Join(d, skillName)); err != nil {
			return err
		}
	}
	fmt.Printf("linked: %s -> ~/.claude/skills, ~/.agents/skills\n", skillDir())

	snip, err := os.ReadFile(snippetPath())
	if err != nil {
		return fmt.Errorf("read %s: %w", snippetPath(), err)
	}
	for _, f := range agentInstructionFiles(home) {
		if err := installSnippet(f, string(snip)); err != nil {
			return err
		}
		fmt.Printf("instructions: the Jet section in %s\n", f)
	}
	return nil
}

// installSnippet puts snip between the jet markers in path, or appends it to
// the file; a backup of the previous content goes to path+".bak-jet" first.
func installSnippet(path, snip string) error {
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(beginMarker) + `.*?` + regexp.QuoteMeta(endMarker) + `\n?`)
	var new []byte
	if len(re.Find(old)) > 0 {
		new = re.ReplaceAll(old, []byte(snip))
	} else if len(strings.TrimSpace(string(old))) > 0 {
		new = []byte(strings.TrimRight(string(old), "\n") + "\n\n" + snip)
	} else {
		new = []byte(snip)
	}
	if old != nil {
		mode := os.FileMode(0o644)
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(path+".bak-jet", old, mode); err != nil {
			return err
		}
	}
	return os.WriteFile(path, new, 0o644)
}

// Uninstall removes jet's links (the jet CLI, the android-jet skill) and
// leaves the instructions in place.
func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if linkTarget(filepath.Join(home, ".local", "bin", "jet")) == jetBin() {
		if err := os.Remove(filepath.Join(home, ".local", "bin", "jet")); err != nil {
			return err
		}
	}
	for _, d := range []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
	} {
		p := filepath.Join(d, skillName)
		if linkTarget(p) == skillDir() {
			if err := os.Remove(p); err != nil {
				return err
			}
		}
	}
	fmt.Println("removed jet's links")
	return nil
}
