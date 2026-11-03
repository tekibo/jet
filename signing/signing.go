package signing

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type KeystoreOptions struct {
	TargetDir    string
	KeystoreName string
	Alias        string
	Password     string
	CommonName   string
	OrgName      string
}

func GenerateKeystore(opts KeystoreOptions) error {
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}

	reader := bufio.NewReader(os.Stdin)

	// Check keytool from JDK
	keytoolPath, err := exec.LookPath("keytool")
	if err != nil {
		javaHome := os.Getenv("JAVA_HOME")
		if javaHome != "" {
			candidate := filepath.Join(javaHome, "bin", "keytool")
			if _, sErr := os.Stat(candidate); sErr == nil {
				keytoolPath = candidate
			}
		}
	}
	if keytoolPath == "" {
		return fmt.Errorf("keytool executable not found in PATH or JAVA_HOME. Please ensure JDK 17+ is installed")
	}

	if opts.KeystoreName == "" {
		fmt.Print("Keystore file name [release.keystore]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = "release.keystore"
		}
		opts.KeystoreName = input
	}

	if opts.Alias == "" {
		fmt.Print("Key alias name [release-key]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = "release-key"
		}
		opts.Alias = input
	}

	if opts.Password == "" {
		for {
			fmt.Print("Enter keystore password (min 6 chars): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if len(input) >= 6 {
				opts.Password = input
				break
			}
			fmt.Println(" [!] Password must be at least 6 characters.")
		}
	}

	if opts.CommonName == "" {
		fmt.Print("First and last name / CN [Android Developer]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = "Android Developer"
		}
		opts.CommonName = input
	}

	if opts.OrgName == "" {
		fmt.Print("Organization name / O [MyCompany]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = "MyCompany"
		}
		opts.OrgName = input
	}

	keystoreFullPath := filepath.Join(opts.TargetDir, opts.KeystoreName)
	if _, err := os.Stat(keystoreFullPath); err == nil {
		fmt.Printf(" [!] Warning: '%s' already exists. Overwrite? [y/N]: ", keystoreFullPath)
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) != "y" {
			return fmt.Errorf("operation cancelled")
		}
		_ = os.Remove(keystoreFullPath)
	}

	dname := fmt.Sprintf("CN=%s, O=%s, C=US", opts.CommonName, opts.OrgName)

	cmd := exec.Command(
		keytoolPath,
		"-genkeypair",
		"-v",
		"-keystore", keystoreFullPath,
		"-alias", opts.Alias,
		"-keyalg", "RSA",
		"-keysize", "2048",
		"-validity", "10000",
		"-storepass", opts.Password,
		"-keypass", opts.Password,
		"-dname", dname,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keytool failed: %s\nOutput: %s", err, string(out))
	}

	fmt.Printf("✓ Release keystore generated at: %s\n", keystoreFullPath)

	// Create keystore.properties in target directory
	propsPath := filepath.Join(opts.TargetDir, "keystore.properties")
	propsContent := fmt.Sprintf("# Android Release Signing Configuration\n# NEVER commit this file or the keystore to git!\nstoreFile=%s\nstorePassword=%s\nkeyAlias=%s\nkeyPassword=%s\n",
		opts.KeystoreName, opts.Password, opts.Alias, opts.Password)

	if err := os.WriteFile(propsPath, []byte(propsContent), 0600); err != nil {
		return fmt.Errorf("failed to write keystore.properties: %w", err)
	}
	fmt.Printf("✓ Created signing configuration: %s\n", propsPath)

	// Ensure .gitignore ignores secrets
	gitignorePath := filepath.Join(opts.TargetDir, ".gitignore")
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content := string(data)
		toAppend := ""
		if !strings.Contains(content, "keystore.properties") {
			toAppend += "\nkeystore.properties\n"
		}
		if !strings.Contains(content, "*.keystore") && !strings.Contains(content, "*.jks") {
			toAppend += "*.keystore\n*.jks\n"
		}
		if toAppend != "" {
			_ = os.WriteFile(gitignorePath, []byte(content+toAppend), 0644)
			fmt.Printf("✓ Updated %s with keystore ignores\n", gitignorePath)
		}
	}

	// Update app/build.gradle.kts to hook up signing if not already present
	gradlePath := filepath.Join(opts.TargetDir, "app", "build.gradle.kts")
	if data, err := os.ReadFile(gradlePath); err == nil {
		gContent := string(data)
		if !strings.Contains(gContent, "keystoreProperties") {
			signingBlock := `
    val keystorePropertiesFile = rootProject.file("keystore.properties")
    val keystoreProperties = java.util.Properties()
    if (keystorePropertiesFile.exists()) {
        keystoreProperties.load(java.io.FileInputStream(keystorePropertiesFile))
    }

    signingConfigs {
        create("release") {
            if (keystorePropertiesFile.exists()) {
                storeFile = rootProject.file(keystoreProperties.getProperty("storeFile"))
                storePassword = keystoreProperties.getProperty("storePassword")
                keyAlias = keystoreProperties.getProperty("keyAlias")
                keyPassword = keystoreProperties.getProperty("keyPassword")
            }
        }
    }
`
			// Inject into android { block
			if idx := strings.Index(gContent, "buildTypes {"); idx != -1 {
				newGContent := gContent[:idx] + signingBlock + "\n    " + gContent[idx:]
				// Enable release signingConfig
				newGContent = strings.Replace(
					newGContent,
					"isMinifyEnabled = false",
					"isMinifyEnabled = true\n            signingConfig = signingConfigs.getByName(\"release\")",
					1,
				)
				_ = os.WriteFile(gradlePath, []byte(newGContent), 0644)
				fmt.Printf("✓ Configured release signing in %s\n", gradlePath)
			}
		}
	}

	fmt.Println("\nSetup complete! You can now build signed release packages:")
	fmt.Println("  ./gradlew assembleRelease   # Builds signed APK")
	fmt.Println("  ./gradlew bundleRelease     # Builds signed Google Play Bundle (AAB)")

	return nil
}
