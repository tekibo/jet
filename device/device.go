package device

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"jet/diagnostics"
	"jet/pkgchanger"
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

func ListDevicesJSON() {
	devices, err := GetDevices()
	if err != nil {
		outputJSON(diagnostics.OutputEnvelope{
			Success: false,
			Command: "devices",
			Error:   err.Error(),
		})
		return
	}
	outputJSON(diagnostics.OutputEnvelope{
		Success: true,
		Command: "devices",
		Data:    devices,
	})
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

func RunApp(targetDir string, explicitDevice string, streamLogs bool) error {
	if targetDir == "" {
		targetDir = "."
	}

	// 1. Detect package
	pkg, err := pkgchanger.DetectPackage(targetDir)
	if err != nil {
		return fmt.Errorf("could not detect package in %s: %w", targetDir, err)
	}

	// 2. Select Device
	devices, err := GetDevices()
	if err != nil {
		return err
	}

	var targetDevice DeviceInfo
	if len(devices) == 0 {
		return fmt.Errorf("no Android devices or emulators connected. Plug in a device or run 'jet emu'")
	}

	if explicitDevice != "" {
		found := false
		for _, d := range devices {
			if d.Serial == explicitDevice {
				targetDevice = d
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("device '%s' not found among connected devices", explicitDevice)
		}
	} else {
		// Pick first active device
		targetDevice = devices[0]
	}

	fmt.Printf("[i] Target device: %s (%s, %s)\n", targetDevice.Serial, targetDevice.Model, targetDevice.Type)

	// 3. Build & Install via Gradle Wrapper
	gradlew := filepath.Join(targetDir, "gradlew")
	if _, err := os.Stat(gradlew); err != nil {
		return fmt.Errorf("gradlew not found in %s", targetDir)
	}

	fmt.Println("[i] Building & installing debug APK...")
	cmd := exec.Command("./gradlew", "installDebug")
	cmd.Dir = targetDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("ANDROID_SERIAL=%s", targetDevice.Serial))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// 4. Launch Main Activity via adb shell am start
	adb, err := FindAdb()
	if err != nil {
		return err
	}

	fmt.Printf("[i] Launching %s/.MainActivity on device...\n", pkg)
	launchCmd := exec.Command(adb, "-s", targetDevice.Serial, "shell", "am", "start", "-n", fmt.Sprintf("%s/.MainActivity", pkg))
	lOut, err := launchCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to launch activity: %s (%w)", string(lOut), err)
	}
	fmt.Printf("✓ %s\n", strings.TrimSpace(string(lOut)))

	// 5. Optional log streaming
	if streamLogs {
		fmt.Printf("\n[i] Streaming logcat for %s (Press Ctrl+C to stop)...\n\n", pkg)
		// Small delay to let process initialize
		time.Sleep(500 * time.Millisecond)

		// Get PID of the launched app
		pidCmd := exec.Command(adb, "-s", targetDevice.Serial, "shell", "pidof", "-s", pkg)
		pidBytes, _ := pidCmd.Output()
		pid := strings.TrimSpace(string(pidBytes))

		var logCmd *exec.Cmd
		if pid != "" {
			logCmd = exec.Command(adb, "-s", targetDevice.Serial, "logcat", fmt.Sprintf("--pid=%s", pid))
		} else {
			// Fallback filter by tag or app name
			logCmd = exec.Command(adb, "-s", targetDevice.Serial, "logcat", "-v", "color", "*:I")
		}

		logCmd.Stdout = os.Stdout
		logCmd.Stderr = os.Stderr
		_ = logCmd.Run()
	}

	return nil
}
