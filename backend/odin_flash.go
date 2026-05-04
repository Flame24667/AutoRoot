package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type OdinFlashResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Log     string `json:"log,omitempty"`
}

// FlashWithOdin executes the Odin flash process
func FlashWithOdin(deviceID string, tarFile string) (*OdinFlashResult, string) {
	// 1. Verify files exist
	if _, err := os.Stat(tarFile); err != nil {
		return nil, fmt.Sprintf("Firmware file not found: %s", tarFile)
	}

	odinPath := getOdinPath()
	if odinPath == "" {
		return nil, "Odin executable not found in resources/odin/"
	}

	// 2. Build Odin command
	// Odin3.exe -device:<serial> -AP:<file> -auto -reboot
	args := []string{
		"-device:" + deviceID,
		"-AP:" + tarFile,
		"-auto",
		"-reboot",
	}

	fmt.Printf("[Odin] Flashing %s with %s\n", deviceID, filepath.Base(tarFile))

	// 3. Execute Odin
	cmd := exec.Command(odinPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	logOutput := stdout.String() + "\n" + stderr.String()

	if err != nil {
		return &OdinFlashResult{
			Success: false,
			Message: fmt.Sprintf("Odin flash failed: %v", err),
			Log:     logOutput,
		}, ""
	}

	// 4. Wait for device to reboot & come back online
	fmt.Println("[Odin] Waiting for device to reboot...")
	time.Sleep(15 * time.Second) // Initial reboot delay

	// Poll until device is back in ADB mode
	for i := 0; i < 30; i++ {
		out, _, _ := runAdb("devices")
		if strings.Contains(out, deviceID) && strings.Contains(out, "device") {
			return &OdinFlashResult{
				Success: true,
				Message: "✅ Flash completed! Device rebooted successfully.",
				Log:     logOutput,
			}, ""
		}
		time.Sleep(5 * time.Second)
	}

	return &OdinFlashResult{
		Success: false,
		Message: "⚠️ Flash may have succeeded, but device didn't return to ADB mode. Check phone screen.",
		Log:     logOutput,
	}, ""
}