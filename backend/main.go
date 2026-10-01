package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Request struct {
	ID      string      `json:"id"`
	Action  string      `json:"action"`
	Payload interface{} `json:"payload,omitempty"`
}

type Response struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		var resp Response
		resp.ID = req.ID

		switch req.Action {
		case "ping":
			resp.Result = "pong"
		case "getDeviceInfo":
			deviceInfo, errStr := getDeviceInfo()
			if errStr != "" {
				resp.Error = errStr
				break
			}
			resp.Result = deviceInfo
			deviceMap, ok := deviceInfo.(map[string]interface{})
			if !ok {
				break
			}
			fwRes, _ := checkFirmware(map[string]interface{}{
				"model":  deviceMap["model"],
				"device": deviceMap["device"],
			})
			fwAvailable := false
			if fwMap, ok := fwRes.(map[string]interface{}); ok {
				if avail, exists := fwMap["available"].(bool); exists {
					fwAvailable = avail
				}
			}
			SaveDeviceHistory(deviceMap, fwAvailable)
		case "rootDevice":
			resp.Result, resp.Error = rootDevice()
		case "checkFirmware":
			resp.Result, resp.Error = checkFirmware(req.Payload)
		case "rebootToDownloadMode":
			deviceID := req.Payload.(map[string]interface{})["deviceID"].(string)
			resp.Result, resp.Error = rebootToDownloadMode(deviceID)
		case "findFirmwareFiles":
			model := req.Payload.(map[string]interface{})["model"].(string)
			resp.Result, resp.Error = findFirmwareFiles(model)
		case "flashWithOdin":
			payload := req.Payload.(map[string]interface{})
			deviceID := payload["deviceID"].(string)
			firmwareFiles := payload["firmwareFiles"].(map[string]interface{})

			fwMap := make(map[string]string)
			for k, v := range firmwareFiles {
				if str, ok := v.(string); ok {
					fwMap[k] = str
				}
			}
			resp.Result, resp.Error = flashWithOdin(deviceID, fwMap)
		case "verifyRootAfterFlash":
			rooted, msg := verifyRootAfterFlash()
			resp.Result = map[string]interface{}{
				"rooted":  rooted,
				"message": msg,
			}
		case "rebootToBootloader":
			deviceID := req.Payload.(map[string]interface{})["deviceID"].(string)
			resp.Result, resp.Error = rebootToBootloader(deviceID)
		case "fastbootFlash":
			payload := req.Payload.(map[string]interface{})
			deviceID := payload["deviceID"].(string)
			bootImage := payload["bootImage"].(string)
			resp.Result, resp.Error = flashWithFastboot(deviceID, bootImage)
		case "waitForFastboot":
			resp.Result, resp.Error = waitForFastboot()
		case "findOnePlusFirmware":
			model := req.Payload.(map[string]interface{})["model"].(string)
			resp.Result, resp.Error = findOnePlusFirmware(model)
		case "flashWithFastboot":
			payload := req.Payload.(map[string]interface{})
			deviceID := payload["deviceID"].(string)
			bootImage := payload["bootImage"].(string)
			resp.Result, resp.Error = flashWithFastboot(deviceID, bootImage)
		case "downloadFirmware":
			resp.Result, resp.Error = downloadFirmware(req.Payload)
		case "listAvailableFirmware":
			resp.Result, resp.Error = listAvailableFirmware(req.Payload)
		case "launchOdinGUI":
			payload, _ := req.Payload.(map[string]interface{})
			apFile, _ := payload["apFile"].(string)
			resp.Result, resp.Error = LaunchOdinGUI(apFile)
		case "odinFlash":
			payload := req.Payload.(map[string]interface{})
			deviceID, _ := payload["deviceID"].(string)
			apFile, _ := payload["apFile"].(string)
			blFile, _ := payload["blFile"].(string)
			cpFile, _ := payload["cpFile"].(string)
			cscFile, _ := payload["cscFile"].(string)
			resp.Result, resp.Error = flashWithOdin(deviceID, map[string]string{
				"AP": apFile, "BL": blFile, "CP": cpFile, "CSC": cscFile,
			})
		case "extractFirmwareToFolder":
			fmt.Println("📦 extractFirmwareToFolder called")
			p := req.Payload.(map[string]interface{})

			zipFile, _ := p["zipFile"].(string)
			brand, _ := p["brand"].(string)
			model, _ := p["model"].(string)
			version, _ := p["version"].(string)
			androidVer, _ := p["androidVersion"].(string)
			binaryBit, _ := p["binaryBit"].(string)

			fmt.Printf("   Params: zip=%s, brand=%s, model=%s\n", zipFile, brand, model)

			folder, files, errStr := ExtractFirmwareToFolder(zipFile, brand, model, version, androidVer, binaryBit)

			if errStr != "" {
				fmt.Printf("   ❌ Error: %s\n", errStr)
				resp.Result = map[string]interface{}{"success": false, "error": errStr}
			} else {
				fmt.Printf("   ✅ Success: %d files\n", len(files))
				resp.Result = map[string]interface{}{
					"success": true,
					"folder":  folder,
					"files":   files,
					"count":   len(files),
				}
			}
		case "handleDroppedFirmware":
			var filePath string
			switch v := req.Payload.(type) {
			case map[string]interface{}:
				filePath, _ = v["filePath"].(string)
			case string:
				filePath = v
			default:
				resp.Error = "Missing or invalid firmware file path"
			}
			if resp.Error == "" {
				resp.Result, resp.Error = handleDroppedFirmware(filePath)
			}
		case "extractFirmware":
			payload := req.Payload.(map[string]interface{})
			zipFile := payload["zipFile"].(string)

			extractDir, files, errStr := ExtractFirmwareToFolder(
				zipFile,
				"", "", "", "", "",
			)

			if errStr != "" {
				resp.Result = map[string]interface{}{"success": false, "error": errStr}
			} else {
				resp.Result = map[string]interface{}{
					"success": true,
					"folder":  extractDir,
					"files":   files,
				}
			}
		case "transferFileToDevice":
			payload := req.Payload.(map[string]interface{})
			deviceID, _ := payload["deviceID"].(string)
			filePath, _ := payload["filePath"].(string)
			destPath, _ := payload["destination"].(string)
			resp.Result, resp.Error = transferFileToDevice(deviceID, filePath, destPath)
		case "getPatchedFileFromDevice":
			payload := req.Payload.(map[string]interface{})
			sourcePath, _ := payload["sourcePath"].(string)
			resp.Result, resp.Error = getPatchedFileFromDevice(sourcePath)
		case "ensureMagiskInstalled":
			deviceID := req.Payload.(map[string]interface{})["deviceID"].(string)
			msg, errStr := ensureMagiskInstalled(deviceID)
			resp.Result = map[string]interface{}{"message": msg}
			resp.Error = errStr
		case "keepDeviceAwake":
			deviceID := req.Payload.(map[string]interface{})["deviceID"].(string)
			keepDeviceAwake(deviceID)
			resp.Result = map[string]interface{}{"success": true}
		case "getLatestMagiskPatchedFile":
			deviceID := req.Payload.(map[string]interface{})["deviceID"].(string)
			resp.Result, resp.Error = getLatestMagiskPatchedFile(deviceID)
		default:
			resp.Error = "unknown action"
		}

		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Scanner error: %v", err)
	}
}

func runAdb(args ...string) (string, string, error) {
	toolsDir := getToolsDir()
	if toolsDir == "" {
		return "", "", fmt.Errorf("unable to resolve bundled Tools directory")
	}
	adbPath := filepath.Join(toolsDir, "platform-tools", "adb.exe")
	if _, err := os.Stat(adbPath); err != nil {
		return "", "", fmt.Errorf("bundled ADB executable not found: %s", adbPath)
	}

	exec.Command(adbPath, "start-server").Run()

	cmd := exec.Command(adbPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func getToolsDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Dir(exePath)), "Tools")
}

func getMarketingName(brand, model string) string {
	// Clean input
	model = strings.TrimSpace(model)
	brand = strings.ToLower(strings.TrimSpace(brand))

	// 1. Try exact match first (fast path)
	if name, ok := deviceNames[model]; ok {
		return name
	}

	// 2. Try prefix matching: iterate through known codes
	//    e.g., "ASUS_AI2401_D" starts with "ASUS_AI2401" → match!
	for code, name := range deviceNames {
		if strings.HasPrefix(model, code) {
			return name
		}
	}

	// 3. Samsung special case: strip "SM-" prefix and retry
	if brand == "samsung" && strings.HasPrefix(model, "SM-") {
		baseModel := strings.TrimPrefix(model, "SM-")

		// Exact match on base
		if name, ok := deviceNames[baseModel]; ok {
			return name
		}
		// Prefix match on base
		for code, name := range deviceNames {
			if strings.HasPrefix(baseModel, code) {
				return name
			}
		}
	}

	// 4. Fallback: return original model code
	return model
}

func getDeviceInfo() (interface{}, string) {
	out, stderr, err := runAdb("devices")
	if err != nil {
		return nil, fmt.Sprintf("ADB failed: %v\n%s", err, stderr)
	}

	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		return nil, "No devices found. Check USB connection & drivers."
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return nil, "Malformed ADB output."
	}

	status := fields[1]
	if status == "unauthorized" {
		return nil, "Device unauthorized. Check phone screen & tap 'Allow USB Debugging'."
	}
	if status == "offline" {
		return nil, "Device offline. Reconnect USB cable."
	}
	if status != "device" {
		return nil, fmt.Sprintf("Unknown device status: %s", status)
	}

	deviceID := fields[0]

	model, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.product.model")
	brand, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.product.brand")
	device, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.product.device")
	marketName, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.product.marketname")
	version, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.build.version.release")
	rootOut, _, _ := runAdb("-s", deviceID, "shell", "su", "-c", "id")

	brand = strings.TrimSpace(brand)
	model = strings.TrimSpace(model)
	device = strings.TrimSpace(device)
	marketName = strings.TrimSpace(marketName)
	version = strings.TrimSpace(version)

	displayName := getMarketingName(strings.ToLower(brand), model)

	if marketName != "" {
		displayName = marketName
	}

	pdaOut, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.build.PDA")
	buildVersion := strings.TrimSpace(pdaOut)

	if buildVersion == "" {
		displayOut, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.build.display.id")
		buildVersion = strings.TrimSpace(displayOut)
	}

	if buildVersion == "" {
		fpOut, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.build.fingerprint")
		buildVersion = strings.TrimSpace(fpOut)
	}

	binaryFull := buildVersion
	binaryBit := "N/A"

	if len(binaryFull) >= 9 {
		binaryBit = string(binaryFull[8])
	}

	return map[string]interface{}{
		"brand":          strings.TrimSpace(brand),
		"model":          strings.TrimSpace(model),
		"displayName":    displayName,
		"device":         device,
		"serial":         deviceID,
		"androidVersion": version,
		"buildVersion":   buildVersion,
		"binaryBit":      binaryBit,
		"rooted":         strings.Contains(rootOut, "uid=0"),
	}, ""
}

func rootDevice() (interface{}, string) {
	return map[string]interface{}{"message": "Root process initiated. Monitor device screen."}, ""
}

func checkFirmware(payload interface{}) (interface{}, string) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil, "Invalid payload"
	}

	model, _ := data["model"].(string)
	brand, _ := data["brand"].(string)

	fwDir := getFirmwareDirectory()
	if fwDir == "" {
		return map[string]interface{}{
			"available": false,
			"message":   "Firmware directory not found",
		}, ""
	}

	fmt.Printf("\n🔍 DEBUG: Looking for model=%s, brand=%s\n", model, brand)
	fmt.Printf("🔍 DEBUG: Firmware dir=%s\n", fwDir)

	// 🔑 Extract model code (SM-A736B → A736B)
	modelCode := model
	if strings.HasPrefix(model, "SM-") {
		modelCode = strings.TrimPrefix(model, "SM-")
	}

	fmt.Printf("🔍 DEBUG: Model code to search: %s\n\n", modelCode)

	var foundFiles []string
	var allFiles []string
	var extractedFiles []string
	var looseFiles []string
	var archiveFiles []string

	// 🔑 List EVERYTHING in firmware directory
	fmt.Println("📁 All files in firmware directory:")
	filepath.Walk(fwDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(fwDir, path)
		fmt.Printf("   - %s\n", relPath)
		allFiles = append(allFiles, path)
		if strings.EqualFold(filepath.Ext(path), ".zip") {
			archiveFiles = append(archiveFiles, path)
		} else if filepath.Dir(relPath) != "." {
			extractedFiles = append(extractedFiles, path)
		} else {
			looseFiles = append(looseFiles, path)
		}
		return nil
	})

	// 🔑 Search for files containing model code
	fmt.Printf("\n🔍 Searching for files containing '%s':\n", modelCode)
	candidates := append(extractedFiles, looseFiles...)
	candidates = append(candidates, archiveFiles...)
	for _, path := range candidates {
		basename := strings.ToUpper(filepath.Base(path))

		if strings.Contains(basename, strings.ToUpper(modelCode)) {
			fmt.Printf("   ✅ MATCH: %s\n", basename)
			foundFiles = append(foundFiles, path)
		} else {
			fmt.Printf("   ❌ skip: %s\n", basename)
		}
	}

	// 🔑 Categorize Samsung files
	if strings.ToLower(brand) == "samsung" && len(foundFiles) > 0 {
		var apFiles, blFiles, cpFiles, cscFiles []string

		for _, f := range foundFiles {
			basename := strings.ToUpper(filepath.Base(f))

			if strings.Contains(basename, "AP_") || strings.HasPrefix(basename, "AP_") {
				apFiles = append(apFiles, f)
				fmt.Printf("   📱 AP file: %s\n", basename)
			} else if strings.Contains(basename, "BL_") || strings.HasPrefix(basename, "BL_") {
				blFiles = append(blFiles, f)
				fmt.Printf("   📱 BL file: %s\n", basename)
			} else if strings.Contains(basename, "CP_") || strings.HasPrefix(basename, "CP_") {
				cpFiles = append(cpFiles, f)
				fmt.Printf("   📱 CP file: %s\n", basename)
			} else if strings.Contains(basename, "CSC_") && !strings.Contains(basename, "HOME_CSC") {
				cscFiles = append(cscFiles, f)
				fmt.Printf("   📱 CSC file: %s\n", basename)
			}
		}

		fmt.Printf("\n📊 Summary: AP=%d, BL=%d, CP=%d, CSC=%d\n",
			len(apFiles), len(blFiles), len(cpFiles), len(cscFiles))

		if len(apFiles) > 0 && len(blFiles) > 0 && len(cpFiles) > 0 && len(cscFiles) > 0 {
			allFiles := append(apFiles, append(blFiles, append(cpFiles, cscFiles...)...)...)
			return map[string]interface{}{
				"available": true,
				"files":     allFiles,
				"count":     len(allFiles),
			}, ""
		}

		missing := []string{}
		if len(apFiles) == 0 {
			missing = append(missing, "AP")
		}
		if len(blFiles) == 0 {
			missing = append(missing, "BL")
		}
		if len(cpFiles) == 0 {
			missing = append(missing, "CP")
		}
		if len(cscFiles) == 0 {
			missing = append(missing, "CSC")
		}

		return map[string]interface{}{
			"available": false,
			"message":   fmt.Sprintf("Missing: %s (Found %d files total)", strings.Join(missing, ", "), len(foundFiles)),
		}, ""
	}

	if len(foundFiles) > 0 {
		return map[string]interface{}{
			"available": true,
			"files":     foundFiles,
		}, ""
	}

	return map[string]interface{}{
		"available": false,
		"message":   fmt.Sprintf("No files found containing '%s'", modelCode),
	}, ""
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFirmwareFile(path string) bool {
	basename := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(basename, ".tar.md5") ||
		strings.HasSuffix(basename, ".img") ||
		strings.HasSuffix(basename, ".zip")
}

func isWrongModel(filename, targetModel string) bool {
	samsungModels := []string{"SM-A", "SM-G", "SM-N", "SM-S", "SM-T", "SM-M"}

	for _, prefix := range samsungModels {
		if strings.Contains(filename, prefix) && !strings.Contains(filename, targetModel) {
			return true
		}
	}

	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
