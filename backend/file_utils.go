package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func handleDroppedFirmware(payload interface{}) (map[string]interface{}, string) {
	// 🔑 Debug: Log exactly what Go receives
	fmt.Printf("📥 Go received payload type: %T, value: %+v\n", payload, payload)

	filePath := ""

	switch v := payload.(type) {
	case map[string]interface{}:
		if val, ok := v["filePath"].(string); ok {
			filePath = strings.TrimSpace(val)
		}
	case map[string]string:
		if val, ok := v["filePath"]; ok {
			filePath = strings.TrimSpace(val)
		}
	case string:
		filePath = strings.TrimSpace(v)
		if filePath != "" && strings.HasPrefix(filePath, "{") {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(filePath), &data); err == nil {
				if val, ok := data["filePath"].(string); ok {
					filePath = strings.TrimSpace(val)
				}
			}
		}
	default:
		return nil, fmt.Sprintf("Invalid payload type: %T. Expected file path, object, or JSON string", payload)
	}

	if filePath == "" {
		return nil, "Missing or empty 'filePath' in payload"
	}

	fmt.Printf("📂 Processing file: %s\n", filePath)

	if !strings.HasSuffix(strings.ToLower(filePath), ".zip") {
		return nil, "Only .zip files are allowed"
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Sprintf("File not found: %s", filePath)
	}

	fwDir := getFirmwareDirectory()
	destPath := filepath.Join(fwDir, filepath.Base(filePath))

	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(destPath)
		name := strings.TrimSuffix(filepath.Base(destPath), ext)
		destPath = filepath.Join(fwDir, fmt.Sprintf("%s_%s%s", name, time.Now().Format("20060102_150405"), ext))
	}

	srcFile, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Sprintf("Failed to open: %v", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Sprintf("Failed to create: %v", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return nil, fmt.Sprintf("Copy failed: %v", err)
	}

	return map[string]interface{}{
		"success": true,
		"path":    destPath,
		"size":    formatBytes(destPath),
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
