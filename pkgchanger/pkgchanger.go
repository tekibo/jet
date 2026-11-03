package pkgchanger

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type RenameOptions struct {
	TargetDir  string
	OldPackage string
	NewPackage string
}

func DetectPackage(targetDir string) (string, error) {
	gradlePath := filepath.Join(targetDir, "app", "build.gradle.kts")
	data, err := os.ReadFile(gradlePath)
	if err != nil {
		// Try Manifest as fallback
		manifestPath := filepath.Join(targetDir, "app", "src", "main", "AndroidManifest.xml")
		mData, mErr := os.ReadFile(manifestPath)
		if mErr != nil {
			return "", fmt.Errorf("could not find app/build.gradle.kts or AndroidManifest.xml in %s", targetDir)
		}
		re := regexp.MustCompile(`package="([^"]+)"`)
		matches := re.FindStringSubmatch(string(mData))
		if len(matches) > 1 {
			return matches[1], nil
		}
		return "", fmt.Errorf("could not detect package name")
	}

	// Search namespace = "..." or applicationId = "..."
	re := regexp.MustCompile(`(?:namespace|applicationId)\s*=\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("could not find namespace or applicationId in %s", gradlePath)
}

func RunRename(opts RenameOptions) error {
	reader := bufio.NewReader(os.Stdin)

	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}

	// 1. Detect or prompt for old package
	if opts.OldPackage == "" {
		detected, err := DetectPackage(opts.TargetDir)
		if err == nil && detected != "" {
			if opts.NewPackage != "" {
				opts.OldPackage = detected
				fmt.Printf("Detected source package: %s\n", detected)
			} else {
				fmt.Printf("Current package detected: %s\n", detected)
				fmt.Printf("Use this package as source? [Y/n]: ")
				ans, _ := reader.ReadString('\n')
				ans = strings.TrimSpace(strings.ToLower(ans))
				if ans == "" || ans == "y" || ans == "yes" {
					opts.OldPackage = detected
				}
			}
		}

		if opts.OldPackage == "" {
			fmt.Print("Enter source/old package name (e.g. com.example.oldapp): ")
			input, _ := reader.ReadString('\n')
			opts.OldPackage = strings.TrimSpace(input)
		}
	}

	// 2. Prompt for new package if not provided
	pkgRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$`)
	for opts.NewPackage == "" {
		fmt.Print("Enter new package name (e.g. com.mycompany.newapp): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if pkgRegex.MatchString(input) {
			opts.NewPackage = input
			break
		}
		fmt.Println(" [!] Invalid package name format. Must be at least two dot-separated segments.")
	}

	if opts.OldPackage == opts.NewPackage {
		return fmt.Errorf("source and destination package names are identical (%s)", opts.OldPackage)
	}

	fmt.Printf("\n[i] Renaming package:\n    From: %s\n    To:   %s\n    In:   %s\n\n",
		opts.OldPackage, opts.NewPackage, opts.TargetDir)

	oldPath := strings.ReplaceAll(opts.OldPackage, ".", "/")
	newPath := strings.ReplaceAll(opts.NewPackage, ".", "/")

	// 3. Move source directories
	srcRoots := []string{
		filepath.Join(opts.TargetDir, "app", "src", "main", "kotlin"),
		filepath.Join(opts.TargetDir, "app", "src", "main", "java"),
		filepath.Join(opts.TargetDir, "app", "src", "test", "kotlin"),
		filepath.Join(opts.TargetDir, "app", "src", "test", "java"),
		filepath.Join(opts.TargetDir, "app", "src", "androidTest", "kotlin"),
		filepath.Join(opts.TargetDir, "app", "src", "androidTest", "java"),
	}

	movedFiles := 0
	for _, root := range srcRoots {
		oldDir := filepath.Join(root, oldPath)
		newDir := filepath.Join(root, newPath)

		if info, err := os.Stat(oldDir); err == nil && info.IsDir() {
			if err := os.MkdirAll(newDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", newDir, err)
			}

			// Move all files and subdirectories from oldDir to newDir
			err := filepath.WalkDir(oldDir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}

				relFile, _ := filepath.Rel(oldDir, p)
				destFile := filepath.Join(newDir, relFile)

				if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
					return err
				}

				if err := moveFile(p, destFile); err != nil {
					return fmt.Errorf("failed to move %s -> %s: %w", p, destFile, err)
				}
				movedFiles++
				return nil
			})
			if err != nil {
				return err
			}

			// Clean up old empty directories
			cleanEmptyDirs(root, oldPath)
		}
	}

	// 4. Update file contents across the project
	updatedFiles := 0
	err := filepath.WalkDir(opts.TargetDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == ".gradle" || base == "build" || base == ".idea" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(p)
		base := filepath.Base(p)
		if ext == ".kt" || ext == ".kts" || ext == ".xml" || ext == ".java" ||
			base == "AGENTS.md" || base == "GEMINI.md" || ext == ".pro" {

			changed, err := replaceInFile(p, opts.OldPackage, opts.NewPackage)
			if err != nil {
				return err
			}
			if changed {
				updatedFiles++
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	fmt.Printf("✓ Package renamed successfully!\n")
	fmt.Printf("  • Source files moved:   %d\n", movedFiles)
	fmt.Printf("  • Project files updated: %d\n", updatedFiles)
	fmt.Println("\nRun './gradlew compileDebugKotlin' to verify your build.")

	return nil
}

func moveFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	in.Close()
	return os.Remove(src)
}

func cleanEmptyDirs(base, subPath string) {
	parts := strings.Split(subPath, "/")
	for i := len(parts); i > 0; i-- {
		dir := filepath.Join(base, filepath.Join(parts[:i]...))
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			os.Remove(dir)
		}
	}
}

func replaceInFile(filePath, oldPkg, newPkg string) (bool, error) {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	content := string(contentBytes)
	if !strings.Contains(content, oldPkg) {
		return false, nil
	}

	newContent := strings.ReplaceAll(content, oldPkg, newPkg)
	if newContent == content {
		return false, nil
	}

	info, _ := os.Stat(filePath)
	mode := os.FileMode(0644)
	if info != nil {
		mode = info.Mode()
	}

	err = os.WriteFile(filePath, []byte(newContent), mode)
	if err != nil {
		return false, err
	}

	return true, nil
}
