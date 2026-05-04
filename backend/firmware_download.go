package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type FirmwareInfo struct {
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	DisplayName    string `json:"displayName"`
	Codename       string `json:"codename"`
	Region         string `json:"region"`
	Version        string `json:"version"`
	AndroidVersion string `json:"androidVersion"`
	URL            string `json:"url"`
	Filename       string `json:"filename"`
	Size           string `json:"size"`
}

// downloadFirmware downloads firmware from URL to firmware directory
func downloadFirmware(payload interface{}) (interface{}, string) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil, "Invalid payload"
	}

	model, _ := data["model"].(string)
	url, _ := data["url"].(string)
	filename, _ := data["filename"].(string)

	if model == "" || url == "" {
		return nil, "Missing model or URL"
	}

	// Get firmware directory
	fwDir := getFirmwareDirectory()
	if fwDir == "" {
		return nil, "Failed to locate firmware directory"
	}

	// Create directory if not exists
	if err := os.MkdirAll(fwDir, 0755); err != nil {
		return nil, fmt.Sprintf("Failed to create firmware directory: %v", err)
	}

	// Full path to save file
	destPath := filepath.Join(fwDir, filename)

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		return map[string]interface{}{
			"success":  true,
			"message":  "Firmware already downloaded",
			"path":     destPath,
			"skipped":  true,
		}, ""
	}

	// Download file
	fmt.Printf("Downloading firmware for %s from %s...\n", model, url)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Sprintf("Download failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Sprintf("HTTP error: %s", resp.Status)
	}

	// Create file
	out, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Sprintf("Failed to create file: %v", err)
	}
	defer out.Close()

	// Copy with progress
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(destPath) // Clean up on error
		return nil, fmt.Sprintf("Download interrupted: %v", err)
	}

	sizeMB := float64(written) / 1024 / 1024

	return map[string]interface{}{
		"success":  true,
		"message":  fmt.Sprintf("Downloaded %.2f MB", sizeMB),
		"path":     destPath,
		"filename": filename,
		"size":     written,
	}, ""
}

// listAvailableFirmware returns list of firmware for a device model
func listAvailableFirmware(payload interface{}) (interface{}, string) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil, "Invalid payload"
	}

	model, _ := data["model"].(string)
	if model == "" {
		return nil, "Model required"
	}

	// Load firmware database
	dbPath := filepath.Join("setup", "firmware-db.json")
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		// Try alternative path
		dbPath = filepath.Join("..", "setup", "firmware-db.json")
		dbData, err = os.ReadFile(dbPath)
		if err != nil {
			return nil, fmt.Sprintf("Failed to load firmware database: %v", err)
		}
	}

	var db struct {
		Devices []struct {
			Brand       string `json:"brand"`
			Model       string `json:"model"`
			DisplayName string `json:"displayName"`
			Codename    string `json:"codename"`
			Firmware    []struct {
				Region         string `json:"region"`
				Version        string `json:"version"`
				AndroidVersion string `json:"androidVersion"`
				URL            string `json:"url"`
				Filename       string `json:"filename"`
				Size           string `json:"size"`
			} `json:"firmware"`
		} `json:"devices"`
	}

	if err := json.Unmarshal(dbData, &db); err != nil {
		return nil, fmt.Sprintf("Invalid firmware database: %v", err)
	}

	// Find matching device
	var available []FirmwareInfo
	for _, device := range db.Devices {
		if strings.Contains(strings.ToUpper(device.Model), strings.ToUpper(model)) ||
		   strings.Contains(strings.ToUpper(model), strings.ToUpper(device.Model)) {
			for _, fw := range device.Firmware {
				available = append(available, FirmwareInfo{
					Brand:          device.Brand,
					Model:          device.Model,
					DisplayName:    device.DisplayName,
					Codename:       device.Codename,
					Region:         fw.Region,
					Version:        fw.Version,
					AndroidVersion: fw.AndroidVersion,
					URL:            fw.URL,
					Filename:       fw.Filename,
					Size:           fw.Size,
				})
			}
		}
	}

	if len(available) == 0 {
		return map[string]interface{}{
			"available": false,
			"message":   "No firmware found for this device",
		}, ""
	}

	return map[string]interface{}{
		"available": true,
		"firmware":  available,
		"count":     len(available),
	}, ""
}

// getFirmwareDirectory returns the firmware directory path
func getFirmwareDirectory() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	exeDir := filepath.Dir(exePath)

	// Check AppData first
	if appData := os.Getenv("APPDATA"); appData != "" {
		fwDir := filepath.Join(appData, "AutoRoot", "firmware")
		if _, err := os.Stat(fwDir); err == nil {
			return fwDir
		}
		// Create it
		os.MkdirAll(fwDir, 0755)
		return fwDir
	}

	// Fallback to local
	localFw := filepath.Join(exeDir, "firmware")
	os.MkdirAll(localFw, 0755)
	return localFw
}