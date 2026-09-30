package main

import (
	"fmt"
	"path/filepath"
)

// ensureMagiskInstalled checks & installs Magisk via ADB
func ensureMagiskInstalled(deviceID string) (string, string) {
	// Check if already installed
	out, _, _ := runAdb("-s", deviceID, "shell", "pm", "path", "com.topjohnwu.magisk")
	if out != "" {
		return "Magisk already installed", ""
	}

	// Find the bundled Magisk APK in Tools.
	toolsDir := getToolsDir()
	if toolsDir == "" {
		return "", "Unable to resolve bundled Tools directory."
	}
	magiskPaths, _ := filepath.Glob(filepath.Join(toolsDir, "Magisk*.apk"))
	if len(magiskPaths) == 0 {
		return "", "Magisk APK not found in Tools/."
	}
	apkPath := magiskPaths[0]

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
