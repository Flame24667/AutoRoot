package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureMagiskInstalled checks & installs Magisk via ADB
func ensureMagiskInstalled(deviceID string) (string, string) {
	// Check if already installed
	out, _, _ := runAdb("-s", deviceID, "shell", "pm", "path", "com.topjohnwu.magisk")
	if out != "" {
		return "Magisk already installed", ""
	}

	// Find bundled Magisk APK (any version-suffixed name, e.g. Magisk-v30.7.apk)
	apkPath := findMagiskAPK()
	if apkPath == "" {
		return "", "Magisk APK not found. Place Magisk*.apk in the app folder or resources/ (or set AUTOROOT_MAGISK_APK)."
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

// findMagiskAPKIn returns the first Magisk*.apk inside dir (case-insensitive).
func findMagiskAPKIn(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasPrefix(name, "magisk") && strings.HasSuffix(name, ".apk") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// findMagiskAPK locates the bundled Magisk APK across common install layouts,
// so names like Magisk-v30.7.apk are found too (not just Magisk.apk).
func findMagiskAPK() string {
	if p := os.Getenv("AUTOROOT_MAGISK_APK"); fileExists(p) {
		return p
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	wd, _ := os.Getwd()

	var dirs []string
	for _, start := range []string{exeDir, wd} {
		abs, err := filepath.Abs(start)
		if err != nil {
			continue
		}
		for i := 0; i < 6; i++ {
			dirs = append(dirs, abs, filepath.Join(abs, "resources"))
			parent := filepath.Dir(abs)
			if parent == abs {
				break
			}
			abs = parent
		}
	}
	dirs = append(dirs,
		filepath.Join(os.Getenv("APPDATA"), "AutoRoot"),
		filepath.Join(os.Getenv("USERPROFILE"), "Downloads"),
		filepath.Join(os.Getenv("USERPROFILE"), "Desktop"),
	)

	for _, d := range dirs {
		if p := findMagiskAPKIn(d); p != "" {
			return p
		}
	}

	fmt.Printf("[Magisk] Magisk*.apk not found. Searched %d folders, including:\n", len(dirs))
	for _, d := range dirs {
		fmt.Printf("   - %s\n", d)
	}
	fmt.Printf("Hint: set AUTOROOT_MAGISK_APK to the full path of the APK\n")
	return ""
}