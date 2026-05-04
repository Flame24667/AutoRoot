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

func getOdinPath() string {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	
	// Try multiple locations
	paths := []string{
		filepath.Join(exeDir, "odin", "Odin3.exe"),
		filepath.Join(exeDir, "resources", "odin", "Odin3.exe"),
		filepath.Join("odin", "Odin3.exe"),
	}
	
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	return ""
}