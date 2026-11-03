package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"jet/diagnostics"
)

type CheckResult struct {
	Name        string
	Found       bool
	Warning     bool
	Description string
	Detail      string
}

type DoctorReport struct {
	Results   []CheckResult
	HasErrors bool
	SDKPath   string
}

func RunDoctor(quiet bool) *DoctorReport {
	report := &DoctorReport{}

	// 1. Check Java / JDK
	javaRes := checkJava()
	report.Results = append(report.Results, javaRes)
	if !javaRes.Found || javaRes.Warning {
		if !javaRes.Found {
			report.HasErrors = true
		}
	}

	// 2. Check Android SDK Location
	sdkRes, sdkPath := checkAndroidSDK()
	report.SDKPath = sdkPath
	report.Results = append(report.Results, sdkRes)
	if !sdkRes.Found {
		report.HasErrors = true
	}

	// 3. Check Command-line tools (sdkmanager)
	cmdlineRes := checkCmdlineTools(sdkPath)
	report.Results = append(report.Results, cmdlineRes)
	if !cmdlineRes.Found {
		report.HasErrors = true
	}

	// 4. Check Platform tools (adb)
	adbRes := checkPlatformTools(sdkPath)
	report.Results = append(report.Results, adbRes)
	if !adbRes.Found {
		report.HasErrors = true
	}

	// 5. Check SDK Platforms
	platformRes := checkPlatforms(sdkPath)
	report.Results = append(report.Results, platformRes)
	if !platformRes.Found {
		report.HasErrors = true
	}

	// 6. Check Build Tools
	btRes := checkBuildTools(sdkPath)
	report.Results = append(report.Results, btRes)
	if !btRes.Found {
		report.HasErrors = true
	}

	// 7. Check Licenses
	licRes := checkLicenses(sdkPath)
	report.Results = append(report.Results, licRes)
	if !licRes.Found {
		report.HasErrors = true
	}

	if !quiet {
		printReport(report)
	}

	return report
}

func RunDoctorJSON() {
	report := RunDoctor(true)
	env := diagnostics.OutputEnvelope{
		Success: !report.HasErrors,
		Command: "doctor",
		Data: map[string]any{
			"has_errors": report.HasErrors,
			"sdk_path":   report.SDKPath,
			"checks":     report.Results,
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(env)
}

func checkJava() CheckResult {
	path, err := exec.LookPath("java")
	if err != nil {
		return CheckResult{
			Name:        "Java / JDK",
			Found:       false,
			Description: "java executable not found in PATH",
			Detail:      "JDK 17 or higher is required to build modern Android Compose apps.",
		}
	}

	out, err := exec.Command("java", "-version").CombinedOutput()
	if err != nil {
		return CheckResult{
			Name:        "Java / JDK",
			Found:       false,
			Description: "Failed to execute 'java -version'",
		}
	}

	verStr := string(out)
	re := regexp.MustCompile(`(?:version|openjdk)\s+"?([0-9]+)(?:\.([0-9]+))?`)
	matches := re.FindStringSubmatch(verStr)
	if len(matches) > 1 {
		major, _ := strconv.Atoi(matches[1])
		if major == 1 && len(matches) > 2 {
			major, _ = strconv.Atoi(matches[2])
		}

		firstLine := strings.Split(verStr, "\n")[0]
		if major < 17 {
			return CheckResult{
				Name:        "Java / JDK",
				Found:       true,
				Warning:     true,
				Description: fmt.Sprintf("Found Java %d (%s) at %s", major, firstLine, path),
				Detail:      "Android Gradle Plugin requires JDK 17+. Please upgrade your JDK.",
			}
		}

		return CheckResult{
			Name:        "Java / JDK",
			Found:       true,
			Description: fmt.Sprintf("%s (%s)", strings.TrimSpace(firstLine), path),
		}
	}

	return CheckResult{
		Name:        "Java / JDK",
		Found:       true,
		Description: fmt.Sprintf("Found java at %s", path),
	}
}

func checkAndroidSDK() (CheckResult, string) {
	home, _ := os.UserHomeDir()

	candidates := []string{
		os.Getenv("ANDROID_HOME"),
		os.Getenv("ANDROID_SDK_ROOT"),
		filepath.Join(home, ".android", "sdk"),
		filepath.Join(home, ".android"),
		filepath.Join(home, "Android", "Sdk"),
		filepath.Join(home, "android-sdk"),
		"/opt/android-sdk",
		"/usr/lib/android-sdk",
	}

	if runtime.GOOS == "darwin" {
		candidates = append(candidates, filepath.Join(home, "Library", "Android", "sdk"))
	} else if runtime.GOOS == "windows" {
		localApp := os.Getenv("LOCALAPPDATA")
		if localApp != "" {
			candidates = append(candidates, filepath.Join(localApp, "Android", "Sdk"))
		}
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			// Check if it looks like an SDK directory (has platform-tools, platforms, or cmdline-tools)
			hasSub := false
			for _, sub := range []string{"platform-tools", "platforms", "cmdline-tools", "build-tools"} {
				if _, err := os.Stat(filepath.Join(p, sub)); err == nil {
					hasSub = true
					break
				}
			}
			if hasSub {
				source := "detected on disk"
				if p == os.Getenv("ANDROID_HOME") {
					source = "via ANDROID_HOME"
				} else if p == os.Getenv("ANDROID_SDK_ROOT") {
					source = "via ANDROID_SDK_ROOT"
				}
				return CheckResult{
					Name:        "Android SDK",
					Found:       true,
					Description: fmt.Sprintf("Found at %s (%s)", p, source),
				}, p
			}
		}
	}

	return CheckResult{
		Name:        "Android SDK",
		Found:       false,
		Description: "Android SDK directory not found in ANDROID_HOME, ~/.android/sdk, or standard paths",
		Detail:      "Set ANDROID_HOME environment variable to your Android SDK folder.",
	}, ""
}

func checkCmdlineTools(sdkPath string) CheckResult {
	// Look in PATH first
	if p, err := exec.LookPath("sdkmanager"); err == nil {
		return CheckResult{
			Name:        "Command-line Tools",
			Found:       true,
			Description: fmt.Sprintf("Found sdkmanager in PATH (%s)", p),
		}
	}

	if sdkPath != "" {
		// Look in $SDK/cmdline-tools/latest/bin/sdkmanager
		p := filepath.Join(sdkPath, "cmdline-tools", "latest", "bin", "sdkmanager")
		if _, err := os.Stat(p); err == nil {
			return CheckResult{
				Name:        "Command-line Tools",
				Found:       true,
				Description: fmt.Sprintf("Found sdkmanager at %s", p),
			}
		}

		// Try globbing cmdline-tools/*/bin/sdkmanager
		matches, _ := filepath.Glob(filepath.Join(sdkPath, "cmdline-tools", "*", "bin", "sdkmanager"))
		if len(matches) > 0 {
			return CheckResult{
				Name:        "Command-line Tools",
				Found:       true,
				Description: fmt.Sprintf("Found sdkmanager at %s", matches[0]),
			}
		}
	}

	return CheckResult{
		Name:        "Command-line Tools",
		Found:       false,
		Description: "sdkmanager not found in PATH or inside Android SDK",
		Detail:      "Download commandlinetools from Google and unpack into <SDK>/cmdline-tools/latest/",
	}
}

func checkPlatformTools(sdkPath string) CheckResult {
	if p, err := exec.LookPath("adb"); err == nil {
		return CheckResult{
			Name:        "Platform Tools",
			Found:       true,
			Description: fmt.Sprintf("Found adb in PATH (%s)", p),
		}
	}

	if sdkPath != "" {
		p := filepath.Join(sdkPath, "platform-tools", "adb")
		if _, err := os.Stat(p); err == nil {
			return CheckResult{
				Name:        "Platform Tools",
				Found:       true,
				Description: fmt.Sprintf("Found adb at %s", p),
			}
		}
	}

	return CheckResult{
		Name:        "Platform Tools",
		Found:       false,
		Description: "adb not found in PATH or inside Android SDK",
		Detail:      "Install via: sdkmanager \"platform-tools\"",
	}
}

func checkPlatforms(sdkPath string) CheckResult {
	if sdkPath == "" {
		return CheckResult{
			Name:        "SDK Platforms",
			Found:       false,
			Description: "Android SDK not located",
		}
	}

	platformsDir := filepath.Join(sdkPath, "platforms")
	entries, err := os.ReadDir(platformsDir)
	if err != nil || len(entries) == 0 {
		return CheckResult{
			Name:        "SDK Platforms",
			Found:       false,
			Description: "No Android platforms installed in " + platformsDir,
			Detail:      "Install via: sdkmanager \"platforms;android-35\" (or android-36)",
		}
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "android-") {
			names = append(names, e.Name())
		}
	}

	if len(names) == 0 {
		return CheckResult{
			Name:        "SDK Platforms",
			Found:       false,
			Description: "No 'android-XX' directory found in " + platformsDir,
			Detail:      "Install via: sdkmanager \"platforms;android-35\"",
		}
	}

	return CheckResult{
		Name:        "SDK Platforms",
		Found:       true,
		Description: fmt.Sprintf("Found: %s", strings.Join(names, ", ")),
	}
}

func checkBuildTools(sdkPath string) CheckResult {
	if sdkPath == "" {
		return CheckResult{
			Name:        "Build Tools",
			Found:       false,
			Description: "Android SDK not located",
		}
	}

	btDir := filepath.Join(sdkPath, "build-tools")
	entries, err := os.ReadDir(btDir)
	if err != nil || len(entries) == 0 {
		return CheckResult{
			Name:        "Build Tools",
			Found:       false,
			Description: "No build-tools installed in " + btDir,
			Detail:      "Install via: sdkmanager \"build-tools;35.0.0\" (or 36.0.0)",
		}
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}

	if len(names) == 0 {
		return CheckResult{
			Name:        "Build Tools",
			Found:       false,
			Description: "No build-tools versions found in " + btDir,
		}
	}

	return CheckResult{
		Name:        "Build Tools",
		Found:       true,
		Description: fmt.Sprintf("Found: %s", strings.Join(names, ", ")),
	}
}

func checkLicenses(sdkPath string) CheckResult {
	if sdkPath == "" {
		return CheckResult{
			Name:        "Android Licenses",
			Found:       false,
			Description: "Android SDK not located",
		}
	}

	licPath := filepath.Join(sdkPath, "licenses", "android-sdk-license")
	if _, err := os.Stat(licPath); err == nil {
		return CheckResult{
			Name:        "Android Licenses",
			Found:       true,
			Description: "Accepted (android-sdk-license found)",
		}
	}

	return CheckResult{
		Name:        "Android Licenses",
		Found:       false,
		Description: "Licenses not yet accepted",
		Detail:      "Accept licenses via: yes | sdkmanager --licenses",
	}
}

func printReport(report *DoctorReport) {
	fmt.Println("=== Android Environment Doctor ===")
	for _, r := range report.Results {
		if r.Found && !r.Warning {
			fmt.Printf(" [✓] %s: %s\n", r.Name, r.Description)
		} else if r.Warning {
			fmt.Printf(" [!] %s: %s\n", r.Name, r.Description)
			if r.Detail != "" {
				fmt.Printf("     -> %s\n", r.Detail)
			}
		} else {
			fmt.Printf(" [✗] %s: %s\n", r.Name, r.Description)
			if r.Detail != "" {
				fmt.Printf("     -> %s\n", r.Detail)
			}
		}
	}
	fmt.Println()

	if !report.HasErrors {
		fmt.Println("• Ready! Your environment has all required tools to build Android Compose apps without Android Studio.")
	} else {
		fmt.Println("• Some tools are missing. Follow the Headless Setup Guide below to install them without Android Studio GUI.")
		PrintHeadlessSetupGuide()
	}
}

func PrintHeadlessSetupGuide() {
	fmt.Println(`
--------------------------------------------------------------------------------
HEADLESS ANDROID SDK SETUP GUIDE (NO ANDROID STUDIO GUI)
--------------------------------------------------------------------------------
Google hides command-line tools at the bottom of their download page.
Here is how to set up the entire Android toolchain in 5 terminal commands:

1. Install OpenJDK 17:
   Ubuntu/Debian: sudo apt install openjdk-17-jdk
   Arch Linux:    sudo pacman -S jdk17-openjdk
   Fedora/RHEL:   sudo dnf install java-17-openjdk-devel

2. Download Google's Command-line Tools:
   Direct Linux CDN:
   https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip
   (macOS: commandlinetools-mac-11076708_latest.zip)

3. Unpack into your SDK location (Google requires the 'latest' folder nesting):
   export ANDROID_HOME="$HOME/.android/sdk"
   mkdir -p "$ANDROID_HOME/cmdline-tools"
   curl -o /tmp/cmdline-tools.zip https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip
   unzip /tmp/cmdline-tools.zip -d /tmp/cmdline-tools-extracted
   mv /tmp/cmdline-tools-extracted/cmdline-tools "$ANDROID_HOME/cmdline-tools/latest"
   rm -rf /tmp/cmdline-tools*

4. Add environment variables to ~/.bashrc or ~/.zshrc:
   export ANDROID_HOME="$HOME/.android/sdk"
   export ANDROID_SDK_ROOT="$HOME/.android/sdk"
   export PATH="$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$PATH"

5. Accept licenses & install platform + build tools:
   yes | sdkmanager --licenses
   sdkmanager "platform-tools" "platforms;android-35" "build-tools;35.0.0"
--------------------------------------------------------------------------------`)
}
