package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// rebootToBootloader reboots device to bootloader/fastboot mode
func rebootToBootloader(deviceID string) (string, string) {
	_, stderr, err := runAdb("-s", deviceID, "reboot", "bootloader")
	if err != nil {
		return "", fmt.Sprintf("Failed to reboot to bootloader: %v\n%s", err, stderr)
	}
	return "Device rebooting to bootloader mode...", ""
}

// waitForFastboot waits for device in fastboot mode
func waitForFastboot() (string, string) {
	maxAttempts := 20
	for i := 0; i < maxAttempts; i++ {
		out, _, err := runFastboot("devices")
		if err == nil && strings.Contains(out, "fastboot") {
			return "Device detected in fastboot mode", ""
		}
		time.Sleep(2 * time.Second)
	}
	return "", "Device not detected in fastboot mode. Please ensure drivers are installed."
}

// flashWithFastboot flashes boot image via fastboot
func flashWithFastboot(deviceID string, bootImage string) (string, string) {
	// Unlock bootloader (this will wipe data!)
	_, stderr, err := runFastboot("flashing", "unlock")
	if err != nil {
		// May already be unlocked
		fmt.Println("Bootloader may already be unlocked:", stderr)
	}
	
	// Flash patched boot image
	_, stderr, err = runFastboot("flash", "boot", bootImage)
	if err != nil {
		return "", fmt.Sprintf("Failed to flash boot image: %v\n%s", err, stderr)
	}
	
	// Relock bootloader (optional)
	// runFastboot("flashing", "lock")
	
	// Reboot
	_, _, _ = runFastboot("reboot")
	
	return "Boot image flashed successfully! Device rebooting...", ""
}

// findOnePlusFirmware searches for OnePlus firmware
func findOnePlusFirmware(model string) (string, string) {
	firmwareDir := getFirmwareDirectory()
	
	// OnePlus uses .zip or .ozip firmware
	patterns := []string{
		fmt.Sprintf("*%s*.zip", model),
		fmt.Sprintf("*OnePlus*9*Pro*.zip"),
		"LE212*.zip",
		"CPH2213*.zip",
	}
	
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(firmwareDir, pattern))
		if len(matches) > 0 {
			return matches[0], ""
		}
	}
	
	return "", fmt.Sprintf("No firmware found for %s in %s", model, firmwareDir)
}

// extractBootImage extracts boot.img from firmware
func extractBootImage(firmwarePath string) (string, string) {
	bootImgPath := filepath.Join(filepath.Dir(firmwarePath), "boot.img")
	
	// Try to extract using Python's payload_dumper or unzip
	// For now, check if boot.img already exists
	if _, err := os.Stat(bootImgPath); err == nil {
		return bootImgPath, ""
	}
	
	return "", "boot.img not found in firmware. Please extract it manually or use payload_dumper."
}

// Helper functions
func runFastboot(args ...string) (string, string, error) {
	var fastbootPath string
	
	// Find fastboot
	if path, err := exec.LookPath("fastboot"); err == nil {
		fastbootPath = path
	} else if runtime.GOOS == "windows" {
		// Check in platform-tools
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			candidate := filepath.Join(appData, "Android", "Sdk", "platform-tools", "fastboot.exe")
			if _, err := os.Stat(candidate); err == nil {
				fastbootPath = candidate
			}
		}
	}
	
	if fastbootPath == "" {
		return "", "", fmt.Errorf("fastboot not found. Install Android Platform Tools")
	}
	
	cmd := exec.Command(fastbootPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	
	return stdout.String(), stderr.String(), err
}