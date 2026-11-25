package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"jet/diagnostics"
	"jet/scaffold"
	"jet/signing"
)

type DeviceInfo struct {
	Serial  string
	Model   string
	Product string
	Type    string // "Physical" or "Emulator"
	Status  string // "device", "offline", "unauthorized"
}

func FindAdb() (string, error) {
	if p, err := exec.LookPath("adb"); err == nil {
		return p, nil
	}
	candidates := []string{
		filepath.Join(os.Getenv("ANDROID_HOME"), "platform-tools", "adb"),
		filepath.Join(os.Getenv("ANDROID_SDK_ROOT"), "platform-tools", "adb"),
	}
	home, _ := os.UserHomeDir()
	candidates = append(candidates,
		filepath.Join(home, ".android", "sdk", "platform-tools", "adb"),
		filepath.Join(home, "Android", "Sdk", "platform-tools", "adb"),
	)

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("adb executable not found. Ensure Android platform-tools is installed")
}

func FindEmulator() (string, error) {
	if p, err := exec.LookPath("emulator"); err == nil {
		return p, nil
	}
	candidates := []string{
		filepath.Join(os.Getenv("ANDROID_HOME"), "emulator", "emulator"),
		filepath.Join(os.Getenv("ANDROID_SDK_ROOT"), "emulator", "emulator"),
	}
	home, _ := os.UserHomeDir()
	candidates = append(candidates,
		filepath.Join(home, ".android", "sdk", "emulator", "emulator"),
		filepath.Join(home, "Android", "Sdk", "emulator", "emulator"),
	)

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("emulator executable not found in Android SDK")
}

func GetDevices() ([]DeviceInfo, error) {
	adb, err := FindAdb()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command(adb, "devices", "-l").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices: %w", err)
	}

	var devices []DeviceInfo
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			serial := parts[0]
			status := parts[1]

			devType := "Physical"
			if strings.HasPrefix(serial, "emulator-") {
				devType = "Emulator"
			}

			model := "Unknown"
			product := ""
			for _, p := range parts[2:] {
				if strings.HasPrefix(p, "model:") {
					model = strings.TrimPrefix(p, "model:")
				} else if strings.HasPrefix(p, "product:") {
					product = strings.TrimPrefix(p, "product:")
				}
			}

			devices = append(devices, DeviceInfo{
				Serial:  serial,
				Model:   model,
				Product: product,
				Type:    devType,
				Status:  status,
			})
		}
	}

	return devices, nil
}

func ListDevices() error {
	devices, err := GetDevices()
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		fmt.Println("No Android devices or emulators currently connected.")
		fmt.Println("Connect a device via USB with USB Debugging enabled, or start an emulator via: jet emu")
		return nil
	}

	fmt.Printf("%-20s %-22s %-12s %s\n", "SERIAL", "MODEL", "TYPE", "STATUS")
	fmt.Println(strings.Repeat("-", 68))
	for _, d := range devices {
		fmt.Printf("%-20s %-22s %-12s %s\n", d.Serial, d.Model, d.Type, d.Status)
	}
	return nil
}

func ListDevicesJSON() error {
	devices, err := GetDevices()
	if err != nil {
		outputJSON(diagnostics.OutputEnvelope{
			Success: false,
			Command: "devices",
			Error:   err.Error(),
		})
		return diagnostics.ExitError{Code: 1, Err: err}
	}
	outputJSON(diagnostics.OutputEnvelope{
		Success: true,
		Command: "devices",
		Data:    devices,
	})
	return nil
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func ListEmulators() ([]string, error) {
	emu, err := FindEmulator()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command(emu, "-list-avds").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list AVDs: %w", err)
	}

	var avds []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			avds = append(avds, line)
		}
	}
	return avds, nil
}

func ShowEmulators() error {
	avds, err := ListEmulators()
	if err != nil {
		return err
	}

	if len(avds) == 0 {
		fmt.Println("No Android Virtual Devices (AVDs) found on this machine.")
		fmt.Println("Create one via: avdmanager create avd -n <name> -k \"system-images;android-35;google_apis;x86_64\"")
		return nil
	}

	fmt.Println("Installed Android Virtual Devices (AVDs):")
	for _, a := range avds {
		fmt.Printf(" • %s\n", a)
	}
	fmt.Println("\nTo launch an emulator:")
	fmt.Printf("  jet emu start %s\n", avds[0])
	fmt.Printf("  jet emu start %s --headless\n", avds[0])
	return nil
}

func StartEmulator(avdName string, headless bool) error {
	emu, err := FindEmulator()
	if err != nil {
		return err
	}

	if avdName == "" {
		avds, err := ListEmulators()
		if err != nil || len(avds) == 0 {
			return fmt.Errorf("no AVD specified and no AVDs found to launch")
		}
		avdName = avds[0]
	}

	args := []string{"-avd", avdName}
	if headless {
		args = append(args, "-no-window")
	}

	fmt.Printf("[i] Launching emulator '%s' (headless: %v)...\n", avdName, headless)
	cmd := exec.Command(emu, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start emulator: %w", err)
	}

	fmt.Println("    Waiting for device to boot...")
	adb, _ := FindAdb()
	if adb != "" {
		_ = exec.Command(adb, "wait-for-device").Run()
		for i := 0; i < 60; i++ {
			out, _ := exec.Command(adb, "shell", "getprop", "sys.boot_completed").CombinedOutput()
			if strings.TrimSpace(string(out)) == "1" {
				fmt.Println("✓ Emulator is booted and ready!")
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}

	fmt.Println("✓ Emulator process started in background.")
	return nil
}

type RunOptions struct {
	TargetDir  string
	Device     string
	Module     string
	StreamLogs bool
	Reinstall  bool
	JSON       bool
	Templates  fs.FS
}

func RunApp(opts RunOptions) error {
	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = "."
	}
	start := time.Now()

	failJSON := func(err error, parsed *diagnostics.GradleParseResult) error {
		if opts.JSON {
			env := diagnostics.OutputEnvelope{
				Success:    false,
				Command:    "run",
				Error:      err.Error(),
				DurationMs: time.Since(start).Milliseconds(),
			}
			if parsed != nil {
				if parsed.SummaryError != "" && parsed.SummaryError != "build failed" {
					env.Error = parsed.SummaryError
				}
				env.FailedTasks = parsed.FailedTasks
				env.Diagnostics = parsed.Diagnostics
				env.RawOutputTail = parsed.RawOutputTail
			}
			outputJSON(env)
			return diagnostics.ExitError{Code: 1, Err: err}
		}
		return err
	}

	// 1. Select Device
	devices, err := GetDevices()
	if err != nil {
		return failJSON(err, nil)
	}

	var readyDevices []DeviceInfo
	for _, d := range devices {
		if d.Status == "device" {
			readyDevices = append(readyDevices, d)
		}
	}

	if len(devices) == 0 {
		return failJSON(fmt.Errorf("no Android devices or emulators connected. Plug in a device or run 'jet emu'"), nil)
	}

	var targetDevice DeviceInfo
	if opts.Device != "" {
		found := false
		for _, d := range devices {
			if d.Serial == opts.Device {
				targetDevice = d
				found = true
				break
			}
		}
		if !found {
			return failJSON(fmt.Errorf("device '%s' not found among connected devices", opts.Device), nil)
		}
		if targetDevice.Status != "device" {
			return failJSON(fmt.Errorf("device '%s' is %s (not ready)", targetDevice.Serial, targetDevice.Status), nil)
		}
	} else if len(readyDevices) > 0 {
		targetDevice = readyDevices[0]
	} else {
		return failJSON(fmt.Errorf("connected device '%s' has status '%s' (authorize USB debugging or wait for boot)", devices[0].Serial, devices[0].Status), nil)
	}

	if !opts.JSON {
		fmt.Printf("[i] Target device: %s (%s, %s)\n", targetDevice.Serial, targetDevice.Model, targetDevice.Type)
	}

	// 2. Ensure Gradle wrapper and debug keystore
	if err := scaffold.EnsureGradleWrapper(opts.Templates, targetDir, opts.JSON); err != nil {
		return failJSON(err, nil)
	}
	if _, err := signing.EnsureDebugKeystore(targetDir, opts.JSON); err != nil {
		return failJSON(err, nil)
	}

	// 3. Build ONLY the target application module APK, then install directly via adb.
	//    Avoids Gradle installing auxiliary application modules (e.g. template APKs)
	//    and avoids AGP ddmlib parallel device contention.
	module := strings.TrimSpace(opts.Module)
	if module == "" {
		module = scaffold.DetectPrimaryAppModule(targetDir)
	} else if !strings.HasPrefix(module, ":") {
		module = ":" + module
	}

	assembleTask := "assembleDebug"
	if module != "" {
		assembleTask = module + ":assembleDebug"
	}

	if !opts.JSON {
		fmt.Printf("[i] Building debug APK via '%s'...\n", assembleTask)
	}
	cmd := exec.Command("./gradlew", assembleTask, "--console=plain")
	cmd.Dir = targetDir

	var outBuf bytes.Buffer
	if opts.JSON {
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		if opts.JSON {
			parsed := diagnostics.ParseGradleResult(outBuf.String())
			return failJSON(fmt.Errorf("build failed: %w", err), &parsed)
		}
		return fmt.Errorf("build failed: %w", err)
	}

	// 4. Locate built APK and detect package + launcher activity
	artifacts := scaffold.FindBuildArtifacts(targetDir, false, false, module)
	if len(artifacts) == 0 {
		return failJSON(fmt.Errorf("build succeeded but no debug APK was found for module '%s'", module), nil)
	}
	apkPath := artifacts[0]

	pkg, activity, err := scaffold.DetectAppLaunchTarget(targetDir, module)
	if err != nil {
		return failJSON(err, nil)
	}

	adb, err := FindAdb()
	if err != nil {
		return failJSON(err, nil)
	}

	if !opts.JSON {
		relAPK, relErr := filepath.Rel(targetDir, apkPath)
		if relErr != nil {
			relAPK = apkPath
		}
		fmt.Printf("[i] Installing %s onto %s...\n", relAPK, targetDevice.Serial)
	}

	installCmd := exec.Command(adb, "-s", targetDevice.Serial, "install", "-r", apkPath)
	installOut, installErr := installCmd.CombinedOutput()
	installStr := strings.TrimSpace(string(installOut))

	if installErr != nil && strings.Contains(installStr, "INSTALL_FAILED_UPDATE_INCOMPATIBLE") && opts.Reinstall {
		if !opts.JSON {
			fmt.Printf("[i] Signature mismatch detected for %s; uninstalling previous build and reinstalling...\n", pkg)
		}
		_, _ = exec.Command(adb, "-s", targetDevice.Serial, "uninstall", pkg).CombinedOutput()
		installCmd = exec.Command(adb, "-s", targetDevice.Serial, "install", apkPath)
		installOut, installErr = installCmd.CombinedOutput()
		installStr = strings.TrimSpace(string(installOut))
	}

	if installErr != nil {
		parsed := diagnostics.ParseGradleResult(installStr)
		msg := fmt.Sprintf("adb install failed: %s", installStr)
		if strings.Contains(installStr, "INSTALL_FAILED_UPDATE_INCOMPATIBLE") {
			msg = fmt.Sprintf("installation failed: existing %s on device has a different signing key (re-run with 'jet run --reinstall')", pkg)
			parsed.Diagnostics = append(parsed.Diagnostics, diagnostics.Diagnostic{
				Severity:   "error",
				Kind:       "signing",
				Message:    installStr,
				Suggestion: "Run 'jet run --reinstall' (or 'adb uninstall " + pkg + "') to replace the previous debug install",
			})
		} else if strings.Contains(installStr, "INSTALL_FAILED_USER_RESTRICTED") {
			parsed.Diagnostics = append(parsed.Diagnostics, diagnostics.Diagnostic{
				Severity:   "error",
				Kind:       "device",
				Message:    installStr,
				Suggestion: "Enable 'Install via USB' in Developer Options on your device and accept the prompt",
			})
		}
		if opts.JSON {
			parsed.SummaryError = msg
			parsed.RawOutputTail = installStr
			return failJSON(fmt.Errorf("%s", msg), &parsed)
		}
		return fmt.Errorf("%s", msg)
	}

	// 5. Launch Main Activity via adb shell am start
	component := fmt.Sprintf("%s/%s", pkg, activity)
	if !opts.JSON {
		fmt.Printf("[i] Launching %s on device...\n", component)
	}
	launchCmd := exec.Command(adb, "-s", targetDevice.Serial, "shell", "am", "start", "-n", component)
	lOut, err := launchCmd.CombinedOutput()
	if err != nil {
		return failJSON(fmt.Errorf("failed to launch activity %s: %s (%w)", component, strings.TrimSpace(string(lOut)), err), nil)
	}

	if opts.JSON {
		relAPK, relErr := filepath.Rel(targetDir, apkPath)
		if relErr != nil {
			relAPK = apkPath
		}
		outputJSON(diagnostics.OutputEnvelope{
			Success:    true,
			Command:    "run",
			DurationMs: time.Since(start).Milliseconds(),
			Data: map[string]any{
				"device":    targetDevice.Serial,
				"module":    module,
				"apk":       relAPK,
				"package":   pkg,
				"activity":  activity,
				"component": component,
			},
		})
		return nil
	}

	fmt.Printf("✓ %s\n", strings.TrimSpace(string(lOut)))

	// 6. Optional log streaming
	if opts.StreamLogs {
		fmt.Printf("\n[i] Streaming logcat for %s (Press Ctrl+C to stop)...\n\n", pkg)
		time.Sleep(500 * time.Millisecond)

		pidCmd := exec.Command(adb, "-s", targetDevice.Serial, "shell", "pidof", "-s", pkg)
		pidBytes, _ := pidCmd.Output()
		pid := strings.TrimSpace(string(pidBytes))

		var logCmd *exec.Cmd
		if pid != "" {
			logCmd = exec.Command(adb, "-s", targetDevice.Serial, "logcat", fmt.Sprintf("--pid=%s", pid))
		} else {
			logCmd = exec.Command(adb, "-s", targetDevice.Serial, "logcat", "-v", "color", "*:I")
		}

		logCmd.Stdout = os.Stdout
		logCmd.Stderr = os.Stderr
		_ = logCmd.Run()
	}

	return nil
}
