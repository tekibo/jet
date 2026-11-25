package signing

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"jet/diagnostics"
)

type KeystoreOptions struct {
	TargetDir    string
	KeystoreName string
	Alias        string
	Password     string
	CommonName   string
	OrgName      string
	Debug        bool
	Force        bool
	JSON         bool
}

func GenerateKeystore(opts KeystoreOptions) error {
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}

	fail := func(err error) error {
		if opts.JSON {
			outputJSON(diagnostics.OutputEnvelope{
				Success: false,
				Command: "keystore",
				Error:   err.Error(),
			})
			return diagnostics.ExitError{Code: 1, Err: err}
		}
		return err
	}

	keytoolPath, err := findKeytool()
	if err != nil {
		return fail(err)
	}

	if opts.Debug {
		return generateDebugMode(keytoolPath, opts, fail)
	}

	reader := bufio.NewReader(os.Stdin)

	if opts.KeystoreName == "" {
		if opts.JSON {
			opts.KeystoreName = "release.keystore"
		} else {
			fmt.Print("Keystore file name [release.keystore]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				input = "release.keystore"
			}
			opts.KeystoreName = input
		}
	}

	if opts.Alias == "" {
		if opts.JSON {
			opts.Alias = "release-key"
		} else {
			fmt.Print("Key alias name [release-key]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				input = "release-key"
			}
			opts.Alias = input
		}
	}

	if opts.Password == "" {
		if opts.JSON {
			return fail(fmt.Errorf("password is required in --json mode (pass --password with at least 6 characters)"))
		}
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
	} else if len(opts.Password) < 6 {
		return fail(fmt.Errorf("password must be at least 6 characters"))
	}

	if opts.CommonName == "" {
		if opts.JSON {
			opts.CommonName = "Android Developer"
		} else {
			fmt.Print("First and last name / CN [Android Developer]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				input = "Android Developer"
			}
			opts.CommonName = input
		}
	}

	if opts.OrgName == "" {
		if opts.JSON {
			opts.OrgName = "MyCompany"
		} else {
			fmt.Print("Organization name / O [MyCompany]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				input = "MyCompany"
			}
			opts.OrgName = input
		}
	}

	keystoreFullPath := filepath.Join(opts.TargetDir, opts.KeystoreName)
	if _, err := os.Stat(keystoreFullPath); err == nil {
		if !opts.Force {
			if opts.JSON {
				return fail(fmt.Errorf("keystore '%s' already exists (pass --force to overwrite)", keystoreFullPath))
			}
			fmt.Printf(" [!] Warning: '%s' already exists. Overwrite? [y/N]: ", keystoreFullPath)
			ans, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(ans)) != "y" {
				return fmt.Errorf("operation cancelled")
			}
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
		return fail(fmt.Errorf("keytool failed: %s\nOutput: %s", err, string(out)))
	}

	if !opts.JSON {
		fmt.Printf("✓ Release keystore generated at: %s\n", keystoreFullPath)
	}

	// Create keystore.properties in target directory
	propsPath := filepath.Join(opts.TargetDir, "keystore.properties")
	propsContent := fmt.Sprintf("# Android Release Signing Configuration\n# NEVER commit this file or the keystore to git!\nstoreFile=%s\nstorePassword=%s\nkeyAlias=%s\nkeyPassword=%s\n",
		opts.KeystoreName, opts.Password, opts.Alias, opts.Password)

	if err := os.WriteFile(propsPath, []byte(propsContent), 0600); err != nil {
		return fail(fmt.Errorf("failed to write keystore.properties: %w", err))
	}
	if !opts.JSON {
		fmt.Printf("✓ Created signing configuration: %s\n", propsPath)
	}

	ensureGitignoreHas(opts.TargetDir, "keystore.properties", "*.keystore", "*.jks")

	// Update app/build.gradle.kts to hook up signing if not already present
	gradlePath := filepath.Join(opts.TargetDir, "app", "build.gradle.kts")
	if data, err := os.ReadFile(gradlePath); err == nil {
		gContent := string(data)
		if !strings.Contains(gContent, "keystoreProperties") && !strings.Contains(gContent, "signingConfigs") {
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
			if idx := strings.Index(gContent, "buildTypes {"); idx != -1 {
				newGContent := gContent[:idx] + signingBlock + "\n    " + gContent[idx:]
				newGContent = strings.Replace(
					newGContent,
					"isMinifyEnabled = false",
					"isMinifyEnabled = true\n            signingConfig = signingConfigs.getByName(\"release\")",
					1,
				)
				_ = os.WriteFile(gradlePath, []byte(newGContent), 0644)
				if !opts.JSON {
					fmt.Printf("✓ Configured release signing in %s\n", gradlePath)
				}
			}
		}
	}

	if opts.JSON {
		outputJSON(diagnostics.OutputEnvelope{
			Success: true,
			Command: "keystore",
			Data: map[string]any{
				"debug":           false,
				"keystore":        opts.KeystoreName,
				"alias":           opts.Alias,
				"properties_file": "keystore.properties",
			},
		})
		return nil
	}

	fmt.Println("\nSetup complete. Build the release packages:")
	fmt.Println("  ./gradlew assembleRelease   # Builds the signed APK")
	fmt.Println("  ./gradlew bundleRelease     # Builds the signed Google Play Bundle (AAB)")

	return nil
}

func generateDebugMode(keytoolPath string, opts KeystoreOptions, fail func(error) error) error {
	name := opts.KeystoreName
	if name == "" || name == "release.keystore" {
		name = "debug.keystore"
	}
	alias := opts.Alias
	if alias == "" || alias == "release-key" {
		alias = "androiddebugkey"
	}
	password := opts.Password
	if password == "" {
		password = "android"
	}
	cn := opts.CommonName
	if cn == "" {
		cn = "Android Debug"
	}
	org := opts.OrgName
	if org == "" {
		org = "Android"
	}

	destPath := filepath.Join(opts.TargetDir, name)
	existed := false
	if _, err := os.Stat(destPath); err == nil {
		if !opts.Force {
			existed = true
		} else {
			_ = os.Remove(destPath)
		}
	}

	if !existed {
		isDefaultDebug := opts.CommonName == "" && opts.OrgName == "" && opts.Password == "" && opts.Alias == ""
		if isDefaultDebug {
			if _, err := provisionStandardDebugKeystore(destPath); err != nil {
				return fail(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fail(fmt.Errorf("failed to create directory for %s: %w", destPath, err))
			}
			dname := fmt.Sprintf("CN=%s, O=%s, C=US", cn, org)
			cmd := exec.Command(
				keytoolPath,
				"-genkeypair",
				"-keystore", destPath,
				"-alias", alias,
				"-keyalg", "RSA",
				"-keysize", "2048",
				"-validity", "10000",
				"-storepass", password,
				"-keypass", password,
				"-dname", dname,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fail(fmt.Errorf("keytool failed: %s\nOutput: %s", err, strings.TrimSpace(string(out))))
			}
		}
	}

	ensureGitignoreHas(opts.TargetDir, filepath.Base(destPath))

	if opts.JSON {
		outputJSON(diagnostics.OutputEnvelope{
			Success: true,
			Command: "keystore",
			Data: map[string]any{
				"debug":    true,
				"keystore": name,
				"alias":    alias,
				"created":  !existed,
			},
		})
		return nil
	}

	if existed {
		fmt.Printf("✓ Debug keystore already exists at: %s\n", destPath)
	} else {
		fmt.Printf("✓ Debug keystore generated at: %s\n", destPath)
	}
	return nil
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
