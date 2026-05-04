package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var historyMutex sync.Mutex

type DeviceHistory struct {
	Serial            string `json:"serial"`
	Brand             string `json:"brand"`
	Model             string `json:"model"`
	DisplayName       string `json:"displayName"`
	Version           string `json:"version"`
	BinaryBit         string `json:"binaryBit"`
	AndroidVersion    string `json:"androidVersion"`
	Rooted            bool   `json:"rooted"`
	FirmwareAvailable bool   `json:"firmwareAvailable"`
	LastDetected      string `json:"lastDetected"`
}

func SaveDeviceHistory(device map[string]interface{}, fwAvailable bool) {
	historyMutex.Lock()
	defer historyMutex.Unlock()

	appData := os.Getenv("APPDATA")
	if appData == "" {
		return
	}
	historyPath := filepath.Join(appData, "AutoRoot", "history.json")

	// Load existing history
	var history struct {
		Devices []DeviceHistory `json:"devices"`
	}
	if data, err := os.ReadFile(historyPath); err == nil {
		json.Unmarshal(data, &history)
	}

	// Safe type assertions
	serial, _ := device["serial"].(string)
	brand, _ := device["brand"].(string)
	model, _ := device["model"].(string)
	displayName, _ := device["displayName"].(string)
	version, _ := device["version"].(string)
	binaryBit, _ := device["binaryBit"].(string)
	androidVer, _ := device["androidVersion"].(string)
	rooted, _ := device["rooted"].(bool)

	newEntry := DeviceHistory{
		Serial:            serial,
		Brand:             brand,
		Model:             model,
		DisplayName:       displayName,
		Version:           version,
		BinaryBit:         binaryBit,
		AndroidVersion:    androidVer,
		Rooted:            rooted,
		FirmwareAvailable: fwAvailable,
		LastDetected:      time.Now().Format("2006-01-02 15:04:05"),
	}

	// Update existing entry or append new one
	updated := false
	for i, dev := range history.Devices {
		if dev.Serial == serial {
			history.Devices[i] = newEntry
			updated = true
			break
		}
	}
	if !updated {
		history.Devices = append(history.Devices, newEntry)
	}

	// Write to file
	os.MkdirAll(filepath.Dir(historyPath), 0755)
	if data, err := json.MarshalIndent(history, "", "  "); err == nil {
		os.WriteFile(historyPath, data, 0644)
	}
}