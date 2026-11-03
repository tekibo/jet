package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"strings"

	"jet/builder"
	"jet/cleaner"
	"jet/depmanager"
	"jet/device"
	"jet/doctor"
	"jet/icon"
	"jet/pkgchanger"
	"jet/scaffold"
	"jet/signing"
	"jet/tester"
)

//go:embed all:templates
var embeddedTemplates embed.FS

const banner = `
     _ _____ _____ 
    | | ____|_   _|
 _  | |  _|   | |  
| |_| | |___  | |  
 \___/|_____| |_|  
Android Jetpack Compose CLI
`

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]
	jsonMode := isJSONRequested()

	switch command {
	case "doctor":
		if jsonMode {
			doctor.RunDoctorJSON()
			return
		}
		fmt.Println(banner)
		doctor.RunDoctor(false)

	case "init":
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		appNameFlag := initCmd.String("name", "", "Application / Project Name")
		pkgFlag := initCmd.String("package", "", "Android package name (e.g. com.example.app)")
		skipDoctor := initCmd.Bool("skip-doctor", false, "Skip preliminary environment check")

		var targetDir string
		var rawArgs []string
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") && targetDir == "" && !strings.Contains(arg, "=") {
				targetDir = arg
			} else {
				rawArgs = append(rawArgs, arg)
			}
		}

		initCmd.Parse(rawArgs)
		if targetDir == "" {
			if len(initCmd.Args()) > 0 {
				targetDir = initCmd.Args()[0]
			} else {
				targetDir = "."
			}
		}

		fmt.Println(banner)

		if !*skipDoctor {
			fmt.Println("[i] Running quick environment doctor...")
			report := doctor.RunDoctor(true)
			if report.HasErrors {
				fmt.Println(" [!] Warning: Some Android tools are missing on your system.")
				fmt.Println("     Run 'jet doctor' to view the headless installation guide.")
			} else {
				fmt.Println(" [✓] Environment check passed.")
			}
			fmt.Println()
		}

		var cfg scaffold.Config
		var err error

		if *appNameFlag != "" && *pkgFlag != "" {
			cfg = scaffold.DefaultConfig()
			cfg.AppName = *appNameFlag
			cfg.PackageName = *pkgFlag
			cfg.PackagePath = strings.ReplaceAll(cfg.PackageName, ".", "/")
			cfg.TargetDir = targetDir
		} else {
			cfg, err = scaffold.PromptConfig(targetDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

		if err := scaffold.Execute(embeddedTemplates, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error during scaffolding: %v\n", err)
			os.Exit(1)
		}

	case "devices":
		if jsonMode {
			device.ListDevicesJSON()
			return
		}
		if err := device.ListDevices(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "emu":
		if len(os.Args) >= 3 && os.Args[2] == "start" {
			avdName := ""
			headless := false
			for _, arg := range os.Args[3:] {
				if arg == "--headless" || arg == "-headless" {
					headless = true
				} else if avdName == "" {
					avdName = arg
				}
			}
			if err := device.StartEmulator(avdName, headless); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting emulator: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := device.ShowEmulators(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		deviceFlag := runCmd.String("device", "", "Explicit Android device/emulator serial ID")
		noLogsFlag := runCmd.Bool("no-logs", false, "Do not stream logcat output after launch")

		var targetDir string
		var rawArgs []string
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") && targetDir == "" && !strings.Contains(arg, "=") {
				targetDir = arg
			} else {
				rawArgs = append(rawArgs, arg)
			}
		}

		runCmd.Parse(rawArgs)
		if targetDir == "" {
			if len(runCmd.Args()) > 0 {
				targetDir = runCmd.Args()[0]
			} else {
				targetDir = "."
			}
		}

		if err := device.RunApp(targetDir, *deviceFlag, !*noLogsFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Run error: %v\n", err)
			os.Exit(1)
		}

	case "build":
		buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
		releaseFlag := buildCmd.Bool("release", false, "Build signed release APK")
		aabFlag := buildCmd.Bool("aab", false, "Build signed Android App Bundle (.aab)")
		jsonFlag := buildCmd.Bool("json", false, "Output machine-readable JSON diagnostics")

		var targetDir string
		var rawArgs []string
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") && targetDir == "" && !strings.Contains(arg, "=") {
				targetDir = arg
			} else {
				rawArgs = append(rawArgs, arg)
			}
		}

		buildCmd.Parse(rawArgs)
		if targetDir == "" {
			if len(buildCmd.Args()) > 0 {
				targetDir = buildCmd.Args()[0]
			} else {
				targetDir = "."
			}
		}

		opts := builder.BuildOptions{
			TargetDir: targetDir,
			Release:   *releaseFlag,
			AAB:       *aabFlag,
			JSON:      *jsonFlag || jsonMode,
		}

		if err := builder.BuildApp(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Build error: %v\n", err)
			os.Exit(1)
		}

	case "add":
		var cleanArgs []string
		for _, arg := range os.Args[2:] {
			if arg != "--json" && arg != "-json" {
				cleanArgs = append(cleanArgs, arg)
			}
		}

		if len(cleanArgs) < 1 {
			if jsonMode {
				depmanager.ListPresetsJSON()
				return
			}
			depmanager.ListPresets()
			os.Exit(0)
		}

		presetName := cleanArgs[0]
		targetDir := "."
		if len(cleanArgs) > 1 {
			targetDir = cleanArgs[1]
		}

		if err := depmanager.AddDependency(targetDir, presetName, jsonMode); err != nil {
			fmt.Fprintf(os.Stderr, "Error adding dependency: %v\n", err)
			os.Exit(1)
		}

	case "rename-pkg":
		renameCmd := flag.NewFlagSet("rename-pkg", flag.ExitOnError)
		toFlag := renameCmd.String("to", "", "New package name (e.g. com.company.newapp)")
		fromFlag := renameCmd.String("from", "", "Current package name (auto-detected if omitted)")

		var targetDir string
		var rawArgs []string
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") && targetDir == "" && !strings.Contains(arg, "=") {
				targetDir = arg
			} else {
				rawArgs = append(rawArgs, arg)
			}
		}

		renameCmd.Parse(rawArgs)
		if targetDir == "" {
			if len(renameCmd.Args()) > 0 {
				targetDir = renameCmd.Args()[0]
			} else {
				targetDir = "."
			}
		}

		fmt.Println(banner)
		opts := pkgchanger.RenameOptions{
			TargetDir:  targetDir,
			OldPackage: *fromFlag,
			NewPackage: *toFlag,
		}

		if err := pkgchanger.RunRename(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error renaming package: %v\n", err)
			os.Exit(1)
		}

	case "keystore":
		keystoreCmd := flag.NewFlagSet("keystore", flag.ExitOnError)
		nameFlag := keystoreCmd.String("name", "release.keystore", "Keystore filename")
		aliasFlag := keystoreCmd.String("alias", "release-key", "Key alias name")

		var targetDir string
		var rawArgs []string
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") && targetDir == "" && !strings.Contains(arg, "=") {
				targetDir = arg
			} else {
				rawArgs = append(rawArgs, arg)
			}
		}

		keystoreCmd.Parse(rawArgs)
		if targetDir == "" {
			if len(keystoreCmd.Args()) > 0 {
				targetDir = keystoreCmd.Args()[0]
			} else {
				targetDir = "."
			}
		}

		fmt.Println(banner)
		opts := signing.KeystoreOptions{
			TargetDir:    targetDir,
			KeystoreName: *nameFlag,
			Alias:        *aliasFlag,
		}

		if err := signing.GenerateKeystore(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating keystore: %v\n", err)
			os.Exit(1)
		}

	case "add-screen":
		if len(os.Args) < 3 {
			fmt.Println("Usage: jet add-screen <ScreenName> [TARGET_DIR]")
			os.Exit(1)
		}
		screenName := os.Args[2]
		targetDir := "."
		if len(os.Args) > 3 {
			targetDir = os.Args[3]
		}

		pkg, err := pkgchanger.DetectPackage(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting package: %v\n", err)
			os.Exit(1)
		}

		if err := scaffold.AddScreen(embeddedTemplates, targetDir, screenName, pkg); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating screen: %v\n", err)
			os.Exit(1)
		}

	case "add-viewmodel":
		if len(os.Args) < 3 {
			fmt.Println("Usage: jet add-viewmodel <ViewModelName> [TARGET_DIR]")
			os.Exit(1)
		}
		vmName := os.Args[2]
		targetDir := "."
		if len(os.Args) > 3 {
			targetDir = os.Args[3]
		}

		pkg, err := pkgchanger.DetectPackage(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting package: %v\n", err)
			os.Exit(1)
		}

		if err := scaffold.AddViewModel(embeddedTemplates, targetDir, vmName, pkg); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating viewmodel: %v\n", err)
			os.Exit(1)
		}

	case "icon":
		iconCmd := flag.NewFlagSet("icon", flag.ExitOnError)
		bgFlag := iconCmd.String("bg", "#1E88E5", "Adaptive icon background hex color (e.g. #1E88E5, #121212)")
		monoFlag := iconCmd.String("monochrome", "", "Path to custom monochrome icon (auto-extracted if omitted)")
		notifFlag := iconCmd.String("notification", "", "Path to custom notification icon (24dp silhouette)")
		splashFlag := iconCmd.String("splash", "", "Path to custom splash screen icon (Android 12+)")
		paddingFlag := iconCmd.Float64("padding", 18.0, "Safe zone padding percentage (default 18%)")
		projectFlag := iconCmd.String("project", "", "Target Android project directory")

		var cleanArgs []string
		for _, a := range os.Args[2:] {
			if a != "--json" && a != "-json" {
				cleanArgs = append(cleanArgs, a)
			}
		}
		_ = iconCmd.Parse(cleanArgs)

		var inputPath string
		targetDir := *projectFlag
		for _, arg := range iconCmd.Args() {
			if inputPath == "" {
				inputPath = arg
			} else if targetDir == "" {
				targetDir = arg
			}
		}

		// If inputPath is actually a project directory, treat it as targetDir
		if inputPath != "" {
			if fi, err := os.Stat(inputPath); err == nil && fi.IsDir() {
				targetDir = inputPath
				inputPath = ""
			}
		}

		if targetDir == "" {
			targetDir = "."
		}

		opts := icon.Options{
			InputPath:        inputPath,
			BgColor:          *bgFlag,
			MonochromePath:   *monoFlag,
			NotificationPath: *notifFlag,
			SplashPath:       *splashFlag,
			ProjectDir:       targetDir,
			PaddingPercent:   *paddingFlag,
			JSON:             jsonMode,
		}

		if err := icon.RunIconGenerator(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating icon suite: %v\n", err)
			os.Exit(1)
		}

	case "test":
		targetDir := "."
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") {
				targetDir = arg
				break
			}
		}

		if err := tester.RunTests(targetDir, jsonMode); err != nil {
			os.Exit(1)
		}

	case "clean":
		cleanCmd := flag.NewFlagSet("clean", flag.ExitOnError)
		allFlag := cleanCmd.Bool("all", false, "Wipe .gradle cache, build/ directories, and stop background daemons")

		var targetDir string
		var rawArgs []string
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") && targetDir == "" && !strings.Contains(arg, "=") {
				targetDir = arg
			} else {
				rawArgs = append(rawArgs, arg)
			}
		}

		cleanCmd.Parse(rawArgs)
		if targetDir == "" {
			targetDir = "."
		}

		if err := cleaner.RunClean(targetDir, *allFlag, jsonMode); err != nil {
			os.Exit(1)
		}

	case "--help", "-h", "help":
		printUsage()

	default:
		// If command starts with an existing folder or name, treat as `init <name>`
		if command != "" && command[0] != '-' {
			fmt.Println(banner)
			cfg, err := scaffold.PromptConfig(command)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := scaffold.Execute(embeddedTemplates, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error during scaffolding: %v\n", err)
				os.Exit(1)
			}
			return
		}
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(banner)
	fmt.Println("Usage:")
	fmt.Println("  jet init [TARGET_DIR]          Interactively scaffold a new Compose project")
	fmt.Println("  jet doctor                     Diagnose your local Android SDK & Java environment")
	fmt.Println("  jet devices                    List connected physical devices & running emulators")
	fmt.Println("  jet emu [start <name>]         List or launch Android Virtual Devices (AVDs)")
	fmt.Println("  jet run [DIR] [--device <id>]  Build, install, launch Activity, & stream logs")
	fmt.Println("  jet build [--release] [--aab]  Build debug APK, release APK, or App Bundle (.aab)")
	fmt.Println("  jet test [TARGET_DIR]          Run unit tests with structured failure parsing")
	fmt.Println("  jet clean [TARGET_DIR] [--all] Clean build caches, daemons, and lock files")
	fmt.Println("  jet icon [FILE] [--bg <hex>]   Generate full icon suite (adaptive, monochrome, splash)")
	fmt.Println("  jet add <preset> [DIR]         Add Compose library (nav, coil, icons, room, ktor, datastore)")
	fmt.Println("  jet add-screen <NAME> [DIR]    Generate a new Material 3 Expressive Composable screen")
	fmt.Println("  jet add-viewmodel <NAME> [DIR] Generate a new UDF ViewModel with StateFlow")
	fmt.Println("  jet rename-pkg [DIR] [--to]    Rename package across source tree and manifests")
	fmt.Println("  jet keystore [TARGET_DIR]      Generate production release keystore & signing config")
	fmt.Println("  jet help                       Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  jet doctor")
	fmt.Println("  jet init my-app")
	fmt.Println("  jet icon ./assets/logo.png --bg \"#1E88E5\"")
	fmt.Println("  jet test my-app --json")
	fmt.Println("  jet clean my-app --all")
	fmt.Println("  jet devices")
	fmt.Println("  jet run my-app")
	fmt.Println("  jet add coil my-app")
	fmt.Println("  jet add-screen ProfileScreen my-app")
	fmt.Println("  jet add-viewmodel Profile my-app")
	fmt.Println("  jet rename-pkg my-app --to \"com.newcompany.app\"")
	fmt.Println("  jet keystore my-app")
	fmt.Println("  jet build --release my-app")
	fmt.Println()
}

func isJSONRequested() bool {
	for _, arg := range os.Args {
		if arg == "--json" || arg == "-json" {
			return true
		}
	}
	return false
}
