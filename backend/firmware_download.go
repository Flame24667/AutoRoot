package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type FirmwareInfo struct {
	Brand              string   `json:"brand"`
	Model              string   `json:"model"`
	DisplayName        string   `json:"displayName"`
	SourceType         string   `json:"sourceType"`
	Region             string   `json:"region"`
	PDA                string   `json:"pda"`
	CSC                string   `json:"csc"`
	CP                 string   `json:"cp"`
	FUSVersionString   string   `json:"fusVersionString"`
	AndroidVersion     string   `json:"androidVersion"`
	BootloaderBinary   string   `json:"bootloaderBinary"`
	ExpectedSizeBytes  int64    `json:"expectedSizeBytes"`
	ExpectedSizeHuman  string   `json:"expectedSizeHuman"`
	MD5                string   `json:"md5"`
	SHA256             string   `json:"sha256"`
	RequiredSlots      []string `json:"requiredSlots"`
	VerificationStatus string   `json:"verificationStatus"`
	Notes              []string `json:"notes,omitempty"`
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
			"success": true,
			"message": "Firmware already downloaded",
			"path":    destPath,
			"skipped": true,
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

	// Load firmware database. The schema changed from "a list of download URLs"
	// to "metadata describing what a correct package looks like", so an entry
	// without a trusted source is reported but never auto-downloaded.
	dbPath := filepath.Join("setup", "firmware-db.json")
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
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
			Device      string `json:"device"`
			Firmware    []struct {
				SourceType         string   `json:"sourceType"`
				Region             string   `json:"region"`
				PDA                string   `json:"pda"`
				CSC                string   `json:"csc"`
				CP                 string   `json:"cp"`
				FUSVersionString   string   `json:"fusVersionString"`
				AndroidVersion     string   `json:"androidVersion"`
				SecurityPatch      string   `json:"securityPatch"`
				BootloaderBinary   string   `json:"bootloaderBinary"`
				ExpectedSizeBytes  int64    `json:"expectedSizeBytes"`
				ExpectedSizeHuman  string   `json:"expectedSizeHuman"`
				MD5                string   `json:"md5"`
				SHA256             string   `json:"sha256"`
				RequiredSlots      []string `json:"requiredSlots"`
				VerificationStatus string   `json:"verificationStatus"`
				Notes              []string `json:"notes"`
			} `json:"firmware"`
		} `json:"devices"`
	}

	if err := json.Unmarshal(dbData, &db); err != nil {
		return nil, fmt.Sprintf("Invalid firmware database: %v", err)
	}

	// Find matching device by exact model code, then by the same prefix rule
	// used for marketing names.
	wantCode := modelCodeOf(model)
	var available []FirmwareInfo
	for _, device := range db.Devices {
		deviceCode := modelCodeOf(device.Model)
		if deviceCode == "" || deviceCode != wantCode {
			continue
		}
		for _, fw := range device.Firmware {
			available = append(available, FirmwareInfo{
				Brand:              device.Brand,
				Model:              device.Model,
				DisplayName:        device.DisplayName,
				SourceType:         fw.SourceType,
				Region:             fw.Region,
				PDA:                fw.PDA,
				CSC:                fw.CSC,
				CP:                 fw.CP,
				FUSVersionString:   fw.FUSVersionString,
				AndroidVersion:     fw.AndroidVersion,
				BootloaderBinary:   fw.BootloaderBinary,
				ExpectedSizeBytes:  fw.ExpectedSizeBytes,
				ExpectedSizeHuman:  fw.ExpectedSizeHuman,
				MD5:                fw.MD5,
				SHA256:             fw.SHA256,
				RequiredSlots:      fw.RequiredSlots,
				VerificationStatus: fw.VerificationStatus,
				Notes:              fw.Notes,
			})
		}
	}

	if len(available) == 0 {
		return map[string]interface{}{
			"available": false,
			"message":   "No firmware metadata found for " + wantCode,
		}, ""
	}

	return map[string]interface{}{
		"available": true,
		"firmware":  available,
		"count":     len(available),
		"message":   "Metadata only. A package still has to be fetched and fully validated before it can be flashed.",
	}, ""
}

// getFirmwareDirectory returns the firmware root. Large packages must never
// land on the system drive, so this resolves to the D: volume (or an explicit
// override) rather than %APPDATA%, which sits on the nearly-full C:.
func getFirmwareDirectory() string {
	return resolveFirmwareRoot()
}
