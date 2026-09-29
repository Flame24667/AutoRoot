package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// magiskAPKNames are the filenames Magisk ships under. Any of them may be
// present depending on how the app was packaged.
var magiskAPKNames = []string{
	"Magisk-v30.7.apk",
	"Magisk.apk",
	"Magisk-v*.apk",
}

// findMagiskAPK locates the bundled Magisk APK. The search walks upward from the
// executable so it works both from a packaged install (next to the binary) and
// from the development tree (the APK sits in the project root, two levels above
// bin/). An explicit override wins over both.
func findMagiskAPK() string {
	if override := strings.TrimSpace(os.Getenv("AUTOROOT_MAGISK_APK")); override != "" {
		if info, err := os.Stat(override); err == nil && !info.IsDir() {
			return override
		}
	}

	if dir := appDir(); dir != "" {
		cur := dir
		for i := 0; i < 5 && cur != ""; i++ {
			for _, name := range magiskAPKNames {
				candidate := filepath.Join(cur, name)
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					return candidate
				}
				if strings.Contains(name, "*") {
					if matches, _ := filepath.Glob(candidate); len(matches) > 0 {
						return matches[0]
					}
				}
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}
	return ""
}

// ensureMagiskInstalled checks & installs Magisk via ADB
func ensureMagiskInstalled(deviceID string) (string, string) {
	// Check if already installed
	out, _, _ := runAdb("-s", deviceID, "shell", "pm", "path", "com.topjohnwu.magisk")
	if strings.Contains(out, "package:") {
		return "Magisk already installed", ""
	}

	apkPath := findMagiskAPK()
	if apkPath == "" {
		return "", "Magisk.apk not found. Place it in the project root or set AUTOROOT_MAGISK_APK."
	}

	// Push the APK, then install it from a shell-side path.
	//
	// `adb install <remote-path>` resolves the path on the host in some adb
	// versions and then reports "failed to stat" even though the file is
	// clearly there. Running `pm install` inside a shell hands the path to the
	// device's own package manager, which always resolves it correctly.
	const remoteAPK = "/data/local/tmp/Magisk.apk"
	if _, pushErr, err := runAdb("-s", deviceID, "push", apkPath, remoteAPK); err != nil {
		return "", fmt.Sprintf("Failed to transfer Magisk: %s", pushErr)
	}
	defer runAdb("-s", deviceID, "shell", "rm", "-f", remoteAPK)

	out, stderr, err := runAdb("-s", deviceID, "shell", "pm", "install", "-r", remoteAPK)
	if err != nil {
		if isInstallViaUSBDisabled(out, stderr) {
			return "", fmt.Sprintf("Install failed: %s. Enable 'Install via USB' in Developer Options and try again.", firstNonEmpty(out, stderr))
		}
		return "", fmt.Sprintf("Install failed: %s", firstNonEmpty(out, stderr))
	}
	if !strings.Contains(out, "Success") {
		return "", fmt.Sprintf("Install did not report success: %s", firstNonEmpty(out, stderr))
	}

	// Confirm the package really landed, so a silent no-op is not reported as a
	// successful install.
	verify, _, _ := runAdb("-s", deviceID, "shell", "pm", "path", "com.topjohnwu.magisk")
	if !strings.Contains(verify, "package:") {
		return "", "install appeared to succeed but com.topjohnwu.magisk is not present on the device"
	}

	return "Magisk installed successfully", ""
}

// isInstallViaUSBDisabled recognises the Android security gate that blocks
// silent installs from a shell.
func isInstallViaUSBDisabled(stdout, stderr string) bool {
	combined := strings.ToLower(stdout + " " + stderr)
	return strings.Contains(combined, "install_failed_verification") ||
		strings.Contains(combined, "install via usb") ||
		strings.Contains(combined, "permission denial") ||
		strings.Contains(combined, "requires install via usb")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return "no output"
}

// keepDeviceAwake prevents screen sleep during rooting
func keepDeviceAwake(deviceID string) {
	runAdb("-s", deviceID, "shell", "settings", "put", "global", "stay_on_while_plugged_in", "3")
	runAdb("-s", deviceID, "shell", "svc", "power", "stayon", "usb")
}
