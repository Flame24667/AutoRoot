package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func handleDroppedFirmware(srcPath string) (map[string]interface{}, string) {
	// Validate extension
	if !strings.HasSuffix(strings.ToLower(srcPath), ".zip") {
		return nil, "Only .zip files are allowed"
	}

	// Verify source exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil, "Source file not found"
	}

	fwDir := getFirmwareDirectory()
	destPath := filepath.Join(fwDir, filepath.Base(srcPath))

	// Handle duplicate names safely
	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(destPath)
		name := strings.TrimSuffix(filepath.Base(destPath), ext)
		timestamp := time.Now().Format("20060102_150405")
		destPath = filepath.Join(fwDir, fmt.Sprintf("%s_%s%s", name, timestamp, ext))
	}

	// Copy file (works cross-platform & across volumes)
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Sprintf("Failed to open source: %v", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Sprintf("Failed to create destination: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return nil, fmt.Sprintf("Failed to copy file: %v", err)
	}

	return map[string]interface{}{
		"success": true,
		"path":    destPath,
		"size":    formatBytes(destFile.Name()),
	}, ""
}

func formatBytes(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "Unknown"
	}
	size := info.Size()
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}