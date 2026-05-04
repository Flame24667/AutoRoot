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

// waitForFastboot waits for device to appear in fastboot mode
func waitForFastboot() (string, string) {
	fmt.Println("Polling for fastboot device...")
	
	maxAttempts := 30 // Increased from 15 to 30 (60 seconds total)
	for i := 0; i < maxAttempts; i++ {
		out, stderr, _ := runFastboot("devices")
		combined := out + stderr
		
		fmt.Printf("Attempt %d/%d: %s\n", i+1, maxAttempts, strings.TrimSpace(combined))
		
		// Check for device (format: "SERIAL\tfastboot")
		if strings.Contains(combined, "fastboot") || 
		   (strings.TrimSpace(combined) != "" && !strings.Contains(combined, "no devices")) {
			return "Fastboot device detected", ""
		}
		
		time.Sleep(2 * time.Second)
	}
	
	return "", "Timed out after 60 seconds. Make sure:\n1. Phone is in bootloader mode (Volume Down + Power)\n2. USB cable is connected\n3. OnePlus/fastboot drivers are installed\n4. Try a different USB port"
}

// findOnePlusFirmware searches for firmware matching the model
func findOnePlusFirmware(model string) (string, string) {
	fwDir := getFirmwareDirectory()
	if fwDir == "" {
		return "", "Firmware directory not found"
	}

	// Search for .img files first
	matches, _ := filepath.Glob(filepath.Join(fwDir, "*.img"))
	if len(matches) > 0 {
		return matches[0], ""
	}
	
	// Fallback to .zip or .bin
	matches, _ = filepath.Glob(filepath.Join(fwDir, "*.zip"))
	if len(matches) > 0 {
		return matches[0], ""
	}

	return "", fmt.Sprintf("No firmware file found for %s", model)
}

// flashWithFastboot flashes boot image via fastboot
func flashWithFastboot(deviceID string, bootImage string) (string, string) {
	// Wait for fastboot device
	_, waitErr := waitForFastboot()
	if waitErr != "" {
		return "", waitErr
	}

	// Flash boot image
	fmt.Printf("Flashing: %s\n", bootImage)
	_, stderr, cmdErr := runFastboot("flash", "boot", bootImage)
	if cmdErr != nil {
		return "", fmt.Sprintf("Failed to flash boot image: %v\n%s", cmdErr, stderr)
	}

	// Reboot device
	runFastboot("reboot")

	return "Boot image flashed successfully! Device rebooting...", ""
}

// runFastboot executes fastboot command
func runFastboot(args ...string) (string, string, error) {
	var fastbootPath string

	// 1. Check system PATH
	if path, err := exec.LookPath("fastboot"); err == nil {
		fastbootPath = path
	} else if runtime.GOOS == "windows" {
		// 2. Check Android SDK platform-tools
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			candidate := filepath.Join(localAppData, "Android", "Sdk", "platform-tools", "fastboot.exe")
			if _, err := os.Stat(candidate); err == nil {
				fastbootPath = candidate
			}
		}
		// 3. Check app bin folder
		if fastbootPath == "" {
			exePath, _ := os.Executable()
			exeDir := filepath.Dir(exePath)
			candidates := []string{
				filepath.Join(exeDir, "bin", "fastboot.exe"),
				filepath.Join(exeDir, "platform-tools", "fastboot.exe"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					fastbootPath = c
					break
				}
			}
		}
	}

	if fastbootPath == "" {
		return "", "", fmt.Errorf("fastboot not found. Please install Android Platform Tools")
	}

	cmd := exec.Command(fastbootPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	return stdout.String(), stderr.String(), err
}