package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractFirmwareToFolder(zipPath string, brand, model, version, androidVer, binaryBit string) (string, []string, string) {
	fmt.Printf("\n🔍 DEBUG: Starting extraction\n")
	fmt.Printf("   Zip: %s\n", zipPath)
	fmt.Printf("   Model: %s\n", model)

	folderName := fmt.Sprintf("%s_%s_%s_%s_%s",
		strings.ToLower(brand),
		strings.ToLower(model),
		strings.ReplaceAll(version, " ", "_"),
		androidVer,
		binaryBit,
	)

	fwDir := getFirmwareDirectory()
	extractDir := filepath.Join(fwDir, folderName)

	fmt.Printf("   Extract dir: %s\n", extractDir)

	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", nil, fmt.Sprintf("Failed to create folder: %v", err)
	}

	var fullPath string
	if filepath.IsAbs(zipPath) {
		fullPath = filepath.Clean(zipPath)
	} else {
		fullPath = filepath.Join(fwDir, zipPath)
	}

	fmt.Printf("   Full path: %s\n", fullPath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", nil, fmt.Sprintf("File not found: %s", fullPath)
	}

	fmt.Println("   Opening zip...")
	r, err := zip.OpenReader(fullPath)
	if err != nil {
		return "", nil, fmt.Sprintf("Failed to open zip: %v", err)
	}
	defer r.Close()

	var extractedFiles []string
	fileCount := 0

	fmt.Printf("   Extracting %d files...\n", len(r.File))

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		destPath := filepath.Join(extractDir, filepath.Base(f.Name))

		if err := extractFile(f, destPath); err != nil {
			return "", nil, fmt.Sprintf("Failed to extract %s: %v", f.Name, err)
		}

		extractedFiles = append(extractedFiles, destPath)
		fileCount++

		if fileCount%10 == 0 {
			fmt.Printf("   Progress: %d/%d files\n", fileCount, len(r.File))
		}
	}

	if len(extractedFiles) == 0 {
		os.RemoveAll(extractDir)
		return "", nil, "No files found in firmware zip"
	}

	fmt.Printf("   ✅ Extracted %d files\n", len(extractedFiles))

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
