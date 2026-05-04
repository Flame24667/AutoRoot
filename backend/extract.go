package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractFirmwareToFolder extracts firmware zip to a named folder and deletes the zip
func ExtractFirmwareToFolder(zipPath string, brand, model, version, androidVer, binaryBit string) (string, []string, string) {
	// 🔑 Create folder name: [brand]_[model]_[version]_[android]_[bit]
	folderName := fmt.Sprintf("%s_%s_%s_%s_%s",
		strings.ToLower(brand),
		strings.ToLower(model),
		strings.ReplaceAll(version, " ", "_"),
		androidVer,
		binaryBit,
	)

	// Get firmware directory
	fwDir := getFirmwareDirectory()
	extractDir := filepath.Join(fwDir, folderName)

	// Create extraction folder
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", nil, fmt.Sprintf("Failed to create folder: %v", err)
	}

	// Resolve full zip path
	fullZipPath := filepath.Join(fwDir, zipPath)

	// Verify zip exists
	if _, err := os.Stat(fullZipPath); os.IsNotExist(err) {
		return "", nil, fmt.Sprintf("Zip file not found: %s", fullZipPath)
	}

	// Open zip file
	r, err := zip.OpenReader(fullZipPath)
	if err != nil {
		return "", nil, fmt.Sprintf("Failed to open zip: %v", err)
	}
	defer r.Close()

	var extractedFiles []string

	// Extract each file
	for _, f := range r.File {
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Create destination path
		destPath := filepath.Join(extractDir, filepath.Base(f.Name))

		// Extract file
		if err := extractFile(f, destPath); err != nil {
			return "", nil, fmt.Sprintf("Failed to extract %s: %v", f.Name, err)
		}

		extractedFiles = append(extractedFiles, destPath)
	}

	if len(extractedFiles) == 0 {
		// Clean up empty folder
		os.RemoveAll(extractDir)
		return "", nil, "No files found in firmware zip"
	}

	// 🔑 Delete the zip file (only .zip files)
	if strings.HasSuffix(strings.ToLower(zipPath), ".zip") {
		if err := os.Remove(fullZipPath); err != nil {
			// Log warning but don't fail - extraction succeeded
			fmt.Printf("Warning: Failed to delete zip %s: %v\n", fullZipPath, err)
		}
	}

	return extractDir, extractedFiles, ""
}

func extractFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}