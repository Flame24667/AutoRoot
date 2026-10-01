package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func transferFileToDevice(deviceID, srcPath, destPath string) (map[string]interface{}, string) {
	// Verify source exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil, fmt.Sprintf("File not found: %s", srcPath)
	}

	if deviceID != "" {
		_, stderr, err := runAdb("-s", deviceID, "shell", "test", "-f", destPath)
		if err == nil {
			return map[string]interface{}{
				"success":       true,
				"alreadyExists": true,
				"message":       fmt.Sprintf("File already exists at %s", destPath),
			}, ""
		}
		if stderr != "" {
			return nil, fmt.Sprintf("ADB file check failed: %v\n%s", err, stderr)
		}
	}

	// Use ADB push
	args := []string{"push", srcPath, destPath}
	if deviceID != "" {
		args = append([]string{"-s", deviceID}, args...)
	}
	_, stderr, err := runAdb(args...)
	if err != nil {
		return nil, fmt.Sprintf("ADB push failed: %v\n%s", err, stderr)
	}

	return map[string]interface{}{
		"success":       true,
		"alreadyExists": false,
		"message":       fmt.Sprintf("Transferred to %s", destPath),
	}, ""
}

func getPatchedFileFromDevice(srcPath string) (map[string]interface{}, string) {
	// Create temp directory for patched file
	tempDir := filepath.Join(os.TempDir(), "autoroot_patched")
	os.MkdirAll(tempDir, 0755)

	destPath := filepath.Join(tempDir, "magisk_patched.tar")

	// Use ADB pull
	_, stderr, err := runAdb("pull", srcPath, destPath)
	if err != nil {
		return nil, fmt.Sprintf("ADB pull failed: %v\n%s", err, stderr)
	}

	return map[string]interface{}{
		"success":   true,
		"localPath": destPath,
		"message":   "Patched file retrieved successfully",
	}, ""
}

// getLatestMagiskPatchedFile finds the newest magisk_patched file in Download
func getLatestMagiskPatchedFile(deviceID string) (map[string]interface{}, string) {
	// List files matching magisk_patched*, sorted newest first
	out, _, _ := runAdb("-s", deviceID, "shell", "ls", "-1", "-t", "/sdcard/Download/magisk_patched*")

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var validFiles []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(strings.ToUpper(line), "/SDCARD/DOWNLOAD/MAGISK_PATCHED") {
			validFiles = append(validFiles, line)
		}
	}

	if len(validFiles) == 0 {
		return nil, "No magisk_patched file found. Please patch the AP file first."
	}

	latestFile := validFiles[0] // -t flag ensures newest is first
	localDir := filepath.Join(os.TempDir(), "autoroot_patched")
	os.MkdirAll(localDir, 0755)
	localPath := filepath.Join(localDir, "patched_ap.tar")

	_, stderr, err := runAdb("-s", deviceID, "pull", latestFile, localPath)
	if err != nil {
		return nil, fmt.Sprintf("Failed to pull: %v\n%s", err, stderr)
	}

	return map[string]interface{}{
		"success":   true,
		"localPath": localPath,
		"source":    latestFile,
	}, ""
}
