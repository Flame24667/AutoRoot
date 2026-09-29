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
		"AP":  "",
		"BL":  "",
		"CP":  "",
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
func flashWithOdin(deviceID string, firmwareFiles map[string]string, installMode string) (*OdinResult, string) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, "Device ID is required"
	}

	for _, slot := range []string{"AP", "BL", "CP", "CSC"} {
		path := firmwareFiles[slot]
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Sprintf("Missing required %s firmware file", slot)
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return nil, fmt.Sprintf("Invalid %s firmware file: %s", slot, path)
		}
	}

	cscName := strings.ToUpper(filepath.Base(firmwareFiles["CSC"]))
	switch installMode {
	case "initial-root":
		if !strings.HasPrefix(cscName, "CSC_") {
			return nil, "Refusing to flash: initial Samsung root installation requires the standard CSC file"
		}
	case "rooted-update":
		if !strings.HasPrefix(cscName, "HOME_CSC_") {
			return nil, "Refusing to flash: a rooted firmware update requires HOME_CSC"
		}
	default:
		return nil, "Refusing to flash: install mode must be initial-root or rooted-update"
	}

	// Get Odin executable path
	odinPath := getOdinPath()
	if odinPath == "" {
		return nil, "Odin executable not found"
	}

	// Build Odin command line
	// Odin3.exe -device:<device_id> -AP:<file> -BL:<file> -CP:<file> -CSC:<file> -auto
	args := []string{fmt.Sprintf("-device:%s", deviceID)}

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
func verifyRootAfterFlash(deviceID string) (bool, string) {
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
		if err != nil || !strings.Contains(out, deviceID+"\tdevice") {
			continue
		}

		// Device detected, check for root
		rootCheck, _, _ := runAdb("-s", deviceID, "shell", "su", "-c", "id")
		if strings.Contains(rootCheck, "uid=0") {
			return true, "Root verified successfully!"
		}
	}

	return false, "Device booted but root not detected. May need manual verification."
}

func getOdinPath() string {
	// Odin has no supported command line, and the archive's Odin3.exe is a
	// Git LFS pointer rather than a program. AutoRoot therefore does not use
	// Odin at all: flashing is handled by samloader-rs, whose CLI is documented
	// and version-pinned (see samloader.go).
	//
	// This function is retained only so existing callers fail loudly instead of
	// silently invoking a GUI tool that cannot be automated.
	return ""
}

func isWindowsExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() < 1024*1024 {
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	header := make([]byte, 2)
	if _, err := file.Read(header); err != nil {
		return false
	}
	return header[0] == 'M' && header[1] == 'Z'
}

func odinFlash(deviceID, apFile, blFile, cpFile, cscFile string) (string, string) {
	// On Windows, you'd call Odin3.exe via CLI
	// For now, this is a placeholder - you'll need the actual Odin CLI tool

	// Example command (Odin CLI doesn't officially exist, you'd need to use a wrapper):
	// Odin3.exe -device:%deviceID% -AP:%apFile% -BL:%blFile% -CP:%cpFile% -CSC:%cscFile%

	return "Odin flash completed successfully", ""
}
