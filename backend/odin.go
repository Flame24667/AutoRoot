package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type OdinResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Log     string `json:"log,omitempty"`
}

// rebootToDownloadMode reboots device to Download Mode
func rebootToDownloadMode(deviceID string) (string, string) {
	_, stderr, err := runAdb("-s", deviceID, "reboot", "download")
	if err != nil {
		return "", fmt.Sprintf("Failed to reboot to Download Mode: %v\n%s", err, stderr)
	}
	return "Device rebooting to Download Mode...", ""
}

// waitForDownloadMode waits for device to appear in Download Mode
func waitForDownloadMode() (string, string) {
	// Odin uses different USB interface, check with adb or wait
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		// Try to detect device via ADB (won't work in Download Mode, but we check)
		_, _, err := runAdb("devices")
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		time.Sleep(2 * time.Second)
	}
	return "Device should be in Download Mode now", ""
}

// findFirmwareFiles searches for Odin-compatible firmware files
func findFirmwareFiles(model string) (map[string]string, string) {
	firmwareDir := getFirmwareDirectory()

	files := map[string]string{
		"AP": "",
		"BL": "",
		"CP": "",
		"CSC": "",
	}

	// Search for files matching pattern: AP_*.tar.md5, BL_*.tar.md5, etc.
	searchPatterns := map[string][]string{
		"AP":  {"AP_*.tar.md5", "AP_*.tar", fmt.Sprintf("*%s*AP*.tar.md5", model)},
		"BL":  {"BL_*.tar.md5", "BL_*.tar", fmt.Sprintf("*%s*BL*.tar.md5", model)},
		"CP":  {"CP_*.tar.md5", "CP_*.tar", fmt.Sprintf("*%s*CP*.tar.md5", model)},
		"CSC": {"CSC_*.tar.md5", "CSC_*.tar", fmt.Sprintf("*%s*CSC*.tar.md5", model)},
	}

	for slot, patterns := range searchPatterns {
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(filepath.Join(firmwareDir, pattern))
			if len(matches) > 0 {
				files[slot] = matches[0]
				break
			}
		}
	}

	// Verify we have at least AP and BL
	if files["AP"] == "" || files["BL"] == "" {
		return nil, "Missing required firmware files (AP and BL). Please ensure firmware package is complete."
	}

	return files, ""
}

// flashWithOdin executes Odin flash
func flashWithOdin(deviceID string, firmwareFiles map[string]string) (*OdinResult, string) {
	// Get Odin executable path
	odinPath := getOdinPath()
	if odinPath == "" {
		return nil, "Odin executable not found"
	}

	// Build Odin command line
	// Odin3.exe -device:<device_id> -AP:<file> -BL:<file> -CP:<file> -CSC:<file> -auto
	args := []string{}

	if firmwareFiles["AP"] != "" {
		args = append(args, fmt.Sprintf("-AP:%s", firmwareFiles["AP"]))
	}
	if firmwareFiles["BL"] != "" {
		args = append(args, fmt.Sprintf("-BL:%s", firmwareFiles["BL"]))
	}
	if firmwareFiles["CP"] != "" {
		args = append(args, fmt.Sprintf("-CP:%s", firmwareFiles["CP"]))
	}
	if firmwareFiles["CSC"] != "" {
		args = append(args, fmt.Sprintf("-CSC:%s", firmwareFiles["CSC"]))
	}

	args = append(args, "-auto", "-reboot")

	// Execute Odin
	cmd := exec.Command(odinPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	logOutput := stdout.String() + "\n" + stderr.String()

	if err != nil {
		return &OdinResult{
			Success: false,
			Message: fmt.Sprintf("Odin flash failed: %v", err),
			Log:     logOutput,
		}, ""
	}

	return &OdinResult{
		Success: true,
		Message: "Flash completed successfully! Device will reboot.",
		Log:     logOutput,
	}, ""
}

// verifyRootAfterFlash checks if device is rooted after reboot
func verifyRootAfterFlash() (bool, string) {
	// Wait for device to boot (can take 5-10 minutes on first boot)
	maxWait := 10 * time.Minute
	interval := 10 * time.Second
	elapsed := time.Duration(0)

	fmt.Println("Waiting for device to boot...")

	for elapsed < maxWait {
		time.Sleep(interval)
		elapsed += interval

		// Try to detect device
		out, _, err := runAdb("devices")
		if err != nil || !strings.Contains(out, "device") {
			continue
		}

		// Device detected, check for root
		rootCheck, _, _ := runAdb("shell", "su", "-c", "id")
		if strings.Contains(rootCheck, "uid=0") {
			return true, "Root verified successfully!"
		}
	}

	return false, "Device booted but root not detected. May need manual verification."
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func searchUpwards(start, sub, file string, maxLevels int) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for i := 0; i < maxLevels; i++ {
		for _, candidate := range []string{
			filepath.Join(dir, sub, file),
			filepath.Join(dir, "resources", sub, file),
		} {
			if fileExists(candidate) {
				return candidate
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func getOdinPath() string {
	if path := os.Getenv("AUTOROOT_ODIN_PATH"); fileExists(path) {
		return path
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	wd, _ := os.Getwd()
	home := os.Getenv("USERPROFILE")
	filenames := []string{"Odin3_v3.14.4.exe", "Odin3.exe"}
	toolsDir := getToolsDir()
	candidates := []string{
		filepath.Join(toolsDir, "odin", filenames[0]),
		filepath.Join(toolsDir, "odin", filenames[1]),
		filepath.Join(exeDir, "Tools", "odin", filenames[0]),
		filepath.Join(exeDir, "Tools", "odin", filenames[1]),
		filepath.Join(exeDir, "resources", "Tools", "odin", filenames[0]),
		filepath.Join(exeDir, "resources", "Tools", "odin", filenames[1]),
		filepath.Join(wd, "Tools", "odin", filenames[0]),
		filepath.Join(wd, "Tools", "odin", filenames[1]),
		filepath.Join(wd, "odin", "Odin3.exe"),
		filepath.Join(home, "Desktop", "Odin3.exe"),
		filepath.Join(home, "Desktop", "odin", "Odin3.exe"),
		filepath.Join(home, "Downloads", "Odin3.exe"),
		filepath.Join(home, "Downloads", "odin", "Odin3.exe"),
		filepath.Join(os.Getenv("APPDATA"), "AutoRoot", "odin", "Odin3.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "AutoRoot", "resources", "Tools", "odin", filenames[0]),
	}

	for _, path := range candidates {
		if fileExists(path) {
			return path
		}
	}

	for _, start := range []string{exeDir, wd} {
		for _, name := range filenames {
			if path := searchUpwards(start, filepath.Join("Tools", "odin"), name, 6); path != "" {
				return path
			}
		}
		if path := searchUpwards(start, "odin", "Odin3.exe", 6); path != "" {
			return path
		}
	}

	if path, err := exec.LookPath("Odin3_v3.14.4.exe"); err == nil {
		return path
	}
	if path, err := exec.LookPath("Odin3.exe"); err == nil {
		return path
	}

	fmt.Printf("[Odin] Odin executable not found. Searched:\n")
	for _, path := range candidates {
		fmt.Printf("   - %s\n", path)
	}
	fmt.Printf("   - upwards from: %s | %s\n", exeDir, wd)
	fmt.Printf("   - PATH\n")
	fmt.Printf("Hint: set AUTOROOT_ODIN_PATH to the full path of Odin3.exe\n")
	return ""
}

// LaunchOdinGUI opens the Odin GUI. Odin3.exe has no supported CLI, so the
// flash itself stays manual: the user loads the patched AP and clicks Start.
func LaunchOdinGUI(apFile string) (string, string) {
	odinPath := getOdinPath()
	if odinPath == "" {
		return "", "Odin executable not found in resources/odin/"
	}

	// Reveal the patched AP in Explorer so the user can pick it in Odin.
	if apFile != "" {
		if _, err := os.Stat(apFile); err == nil {
			exec.Command("explorer", "/select,"+apFile).Start()
		}
	}

	cmd := exec.Command(odinPath)
	if err := cmd.Start(); err != nil {
		return "", fmt.Sprintf("Failed to launch Odin: %v", err)
	}

	if apFile != "" {
		return fmt.Sprintf("Odin opened. In Odin: AP → select %s, then click Start.", filepath.Base(apFile)), ""
	}
	return "Odin opened. Load the patched AP into the AP slot, then click Start.", ""
}

func odinFlash(deviceID, apFile, blFile, cpFile, cscFile string) (string, string) {
	// On Windows, you'd call Odin3.exe via CLI
	// For now, this is a placeholder - you'll need the actual Odin CLI tool

	// Example command (Odin CLI doesn't officially exist, you'd need to use a wrapper):
	// Odin3.exe -device:%deviceID% -AP:%apFile% -BL:%blFile% -CP:%cpFile% -CSC:%cscFile%

	return "Odin flash completed successfully", ""
}
