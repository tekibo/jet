package depmanager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jet/diagnostics"
)

type DependencyPreset struct {
	Name        string
	Description string
	Versions    map[string]string // key -> version
	Libraries   map[string]string // alias -> toml definition
	GradleDeps  []string          // implementation lines
}

var Presets = map[string]DependencyPreset{
	"navigation": {
		Name:        "navigation",
		Description: "Jetpack Navigation Compose for type-safe screen navigation",
		Versions: map[string]string{
			"navigationCompose": "2.8.8",
		},
		Libraries: map[string]string{
			"androidx-navigation-compose": `{ group = "androidx.navigation", name = "navigation-compose", version.ref = "navigationCompose" }`,
		},
		GradleDeps: []string{
			"implementation(libs.androidx.navigation.compose)",
		},
	},
	"coil": {
		Name:        "coil",
		Description: "Coil 3 asynchronous image loading for Compose",
		Versions: map[string]string{
			"coil": "3.1.0",
		},
		Libraries: map[string]string{
			"coil-compose":        `{ group = "io.coil-kt.coil3", name = "coil-compose", version.ref = "coil" }`,
			"coil-network-okhttp": `{ group = "io.coil-kt.coil3", name = "coil-network-okhttp", version.ref = "coil" }`,
		},
		GradleDeps: []string{
			"implementation(libs.coil.compose)",
			"implementation(libs.coil.network.okhttp)",
		},
	},
	"icons": {
		Name:        "icons",
		Description: "Material 3 Extended Icons for Compose",
		Versions:    map[string]string{},
		Libraries: map[string]string{
			"androidx-compose-material-icons-extended": `{ group = "androidx.compose.material", name = "material-icons-extended" }`,
		},
		GradleDeps: []string{
			"implementation(libs.androidx.compose.material.icons.extended)",
		},
	},
	"viewmodel": {
		Name:        "viewmodel",
		Description: "Lifecycle ViewModel integration for Jetpack Compose",
		Versions:    map[string]string{},
		Libraries: map[string]string{
			"androidx-lifecycle-viewmodel-compose": `{ group = "androidx.lifecycle", name = "lifecycle-viewmodel-compose", version.ref = "lifecycleRuntimeKtx" }`,
		},
		GradleDeps: []string{
			"implementation(libs.androidx.lifecycle.viewmodel.compose)",
		},
	},
	"room": {
		Name:        "room",
		Description: "Room SQLite persistence library & coroutines extension",
		Versions: map[string]string{
			"room": "2.6.1",
		},
		Libraries: map[string]string{
			"androidx-room-runtime": `{ group = "androidx.room", name = "room-runtime", version.ref = "room" }`,
			"androidx-room-ktx":     `{ group = "androidx.room", name = "room-ktx", version.ref = "room" }`,
		},
		GradleDeps: []string{
			"implementation(libs.androidx.room.runtime)",
			"implementation(libs.androidx.room.ktx)",
		},
	},
	"ktor": {
		Name:        "ktor",
		Description: "Ktor HTTP client for Kotlin coroutines & network requests",
		Versions: map[string]string{
			"ktor": "3.1.1",
		},
		Libraries: map[string]string{
			"ktor-client-core":   `{ group = "io.ktor", name = "ktor-client-core", version.ref = "ktor" }`,
			"ktor-client-okhttp": `{ group = "io.ktor", name = "ktor-client-okhttp", version.ref = "ktor" }`,
		},
		GradleDeps: []string{
			"implementation(libs.ktor.client.core)",
			"implementation(libs.ktor.client.okhttp)",
		},
	},
	"serialization": {
		Name:        "serialization",
		Description: "Kotlinx Serialization JSON library",
		Versions: map[string]string{
			"kotlinxSerializationJson": "1.8.0",
		},
		Libraries: map[string]string{
			"kotlinx-serialization-json": `{ group = "org.jetbrains.kotlinx", name = "kotlinx-serialization-json", version.ref = "kotlinxSerializationJson" }`,
		},
		GradleDeps: []string{
			"implementation(libs.kotlinx.serialization.json)",
		},
	},
	"datastore": {
		Name:        "datastore",
		Description: "Jetpack DataStore Preferences for reactive key-value storage",
		Versions: map[string]string{
			"datastore": "1.1.3",
		},
		Libraries: map[string]string{
			"androidx-datastore-preferences": `{ group = "androidx.datastore", name = "datastore-preferences", version.ref = "datastore" }`,
		},
		GradleDeps: []string{
			"implementation(libs.androidx.datastore.preferences)",
		},
	},
	"camera": {
		Name:        "camera",
		Description: "AndroidX CameraX (Camera2, Lifecycle, and View)",
		Versions: map[string]string{
			"camerax": "1.4.1",
		},
		Libraries: map[string]string{
			"androidx-camera-camera2":   `{ group = "androidx.camera", name = "camera-camera2", version.ref = "camerax" }`,
			"androidx-camera-lifecycle": `{ group = "androidx.camera", name = "camera-lifecycle", version.ref = "camerax" }`,
			"androidx-camera-view":      `{ group = "androidx.camera", name = "camera-view", version.ref = "camerax" }`,
		},
		GradleDeps: []string{
			"implementation(libs.androidx.camera.camera2)",
			"implementation(libs.androidx.camera.lifecycle)",
			"implementation(libs.androidx.camera.view)",
		},
	},
	"workmanager": {
		Name:        "workmanager",
		Description: "AndroidX WorkManager for deferrable, guaranteed background work",
		Versions: map[string]string{
			"work": "2.10.0",
		},
		Libraries: map[string]string{
			"androidx-work-runtime-ktx": `{ group = "androidx.work", name = "work-runtime-ktx", version.ref = "work" }`,
		},
		GradleDeps: []string{
			"implementation(libs.androidx.work.runtime.ktx)",
		},
	},
}

func AddDependency(targetDir, presetName string, isJSON bool) error {
	if targetDir == "" {
		targetDir = "."
	}

	presetName = strings.ToLower(presetName)
	// Aliases
	if presetName == "nav" {
		presetName = "navigation"
	} else if presetName == "images" || presetName == "image" {
		presetName = "coil"
	} else if presetName == "icon" {
		presetName = "icons"
	} else if presetName == "json" {
		presetName = "serialization"
	} else if presetName == "prefs" || presetName == "preferences" {
		presetName = "datastore"
	} else if presetName == "camerax" {
		presetName = "camera"
	} else if presetName == "work" {
		presetName = "workmanager"
	}

	preset, exists := Presets[presetName]
	if !exists {
		if isJSON {
			outputJSON(diagnostics.OutputEnvelope{
				Success: false,
				Command: "add",
				Error:   fmt.Sprintf("unknown preset '%s'", presetName),
			})
			return nil
		}
		fmt.Printf("Unknown dependency preset: '%s'\n\n", presetName)
		ListPresets()
		return fmt.Errorf("preset '%s' not found", presetName)
	}

	tomlPath := filepath.Join(targetDir, "gradle", "libs.versions.toml")
	gradlePath := filepath.Join(targetDir, "app", "build.gradle.kts")

	if _, err := os.Stat(tomlPath); err != nil {
		if isJSON {
			outputJSON(diagnostics.OutputEnvelope{
				Success: false,
				Command: "add",
				Error:   fmt.Sprintf("libs.versions.toml not found at %s", tomlPath),
			})
			return nil
		}
		return fmt.Errorf("libs.versions.toml not found at %s", tomlPath)
	}
	if _, err := os.Stat(gradlePath); err != nil {
		if isJSON {
			outputJSON(diagnostics.OutputEnvelope{
				Success: false,
				Command: "add",
				Error:   fmt.Sprintf("app/build.gradle.kts not found at %s", gradlePath),
			})
			return nil
		}
		return fmt.Errorf("app/build.gradle.kts not found at %s", gradlePath)
	}

	// 1. Update libs.versions.toml
	if err := updateToml(tomlPath, preset); err != nil {
		return err
	}

	// 2. Update app/build.gradle.kts
	if err := updateBuildGradle(gradlePath, preset); err != nil {
		return err
	}

	if isJSON {
		outputJSON(diagnostics.OutputEnvelope{
			Success: true,
			Command: "add",
			Data: map[string]any{
				"preset":            preset.Name,
				"target_dir":        targetDir,
				"added_dependencies": preset.GradleDeps,
			},
		})
		return nil
	}

	fmt.Printf("✓ Added '%s' to %s!\n", preset.Name, targetDir)
	for _, d := range preset.GradleDeps {
		fmt.Printf("  • %s\n", d)
	}
	fmt.Println("\nRun './gradlew compileDebugKotlin' to download and sync dependencies.")
	return nil
}

func ListPresetsJSON() {
	outputJSON(diagnostics.OutputEnvelope{
		Success: true,
		Command: "add",
		Data:    Presets,
	})
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func ListPresets() {
	fmt.Println("Available dependency presets (run: jet add <preset>):")
	fmt.Println("----------------------------------------------------------------------")
	fmt.Printf("%-16s %s\n", "PRESET", "DESCRIPTION")
	fmt.Println("----------------------------------------------------------------------")
	for key, p := range Presets {
		fmt.Printf("%-16s %s\n", key, p.Description)
	}
	fmt.Println("----------------------------------------------------------------------")
}

func updateToml(path string, preset DependencyPreset) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)

	// Inject versions
	for vKey, vVal := range preset.Versions {
		if !strings.Contains(content, vKey+" =") {
			// Insert under [versions]
			idx := strings.Index(content, "[versions]")
			if idx != -1 {
				newlineIdx := strings.Index(content[idx:], "\n")
				if newlineIdx != -1 {
					insertPos := idx + newlineIdx + 1
					content = content[:insertPos] + fmt.Sprintf("%s = \"%s\"\n", vKey, vVal) + content[insertPos:]
				}
			}
		}
	}

	// Inject libraries
	for libKey, libVal := range preset.Libraries {
		if !strings.Contains(content, libKey+" =") {
			idx := strings.Index(content, "[libraries]")
			if idx != -1 {
				newlineIdx := strings.Index(content[idx:], "\n")
				if newlineIdx != -1 {
					insertPos := idx + newlineIdx + 1
					content = content[:insertPos] + fmt.Sprintf("%s = %s\n", libKey, libVal) + content[insertPos:]
				}
			}
		}
	}

	return os.WriteFile(path, []byte(content), 0644)
}

func updateBuildGradle(path string, preset DependencyPreset) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)

	depIdx := strings.Index(content, "dependencies {")
	if depIdx == -1 {
		return fmt.Errorf("could not find 'dependencies {' block in %s", path)
	}

	for _, depLine := range preset.GradleDeps {
		if !strings.Contains(content, depLine) {
			// Insert after dependencies {
			newlineIdx := strings.Index(content[depIdx:], "\n")
			if newlineIdx != -1 {
				insertPos := depIdx + newlineIdx + 1
				content = content[:insertPos] + fmt.Sprintf("    %s\n", depLine) + content[insertPos:]
			}
		}
	}

	return os.WriteFile(path, []byte(content), 0644)
}
