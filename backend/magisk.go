package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// ensureMagiskInstalled checks & installs Magisk via ADB
func ensureMagiskInstalled(deviceID string) (string, string) {
	// Check if already installed
	out, _, _ := runAdb("-s", deviceID, "shell", "pm", "path", "com.topjohnwu.magisk")
	if out != "" {
		return "Magisk already installed", ""
	}

	// Find bundled Magisk APK
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	magiskPaths := []string{
		filepath.Join(exeDir, "Magisk.apk"),
		filepath.Join(exeDir, "resources", "Magisk.apk"),
		filepath.Join(exeDir, "..", "Magisk.apk"),
	}

	var apkPath string
	for _, p := range magiskPaths {
		if _, err := os.Stat(p); err == nil {
			apkPath = p
			break
		}
	}

	if apkPath == "" {
		return "", "Magisk.apk not found. Place it in the app folder or resources/."
	}

	// Push & install
	runAdb("-s", deviceID, "push", apkPath, "/data/local/tmp/Magisk.apk")
	_, stderr, err := runAdb("-s", deviceID, "install", "-r", "/data/local/tmp/Magisk.apk")
	runAdb("-s", deviceID, "shell", "rm", "/data/local/tmp/Magisk.apk")
	
	if err != nil {
		return "", fmt.Sprintf("Install failed: %s. Enable 'Install via USB' in Developer Options.", stderr)
	}

	return "Magisk installed successfully", ""
}

// keepDeviceAwake prevents screen sleep during rooting
func keepDeviceAwake(deviceID string) {
	runAdb("-s", deviceID, "shell", "settings", "put", "global", "stay_on_while_plugged_in", "3")
	runAdb("-s", deviceID, "shell", "svc", "power", "stayon", "usb")
}