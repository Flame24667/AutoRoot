package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"path/filepath"
	// "regexp"
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
			
			// Convert to map[string]string
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
				"rooted": rooted,
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
		case "odinFlash":
			payload := req.Payload.(map[string]interface{})
			deviceID := payload["deviceID"].(string)
			tarFile := payload["tarFile"].(string)
			resp.Result, resp.Error = FlashWithOdin(deviceID, tarFile)
		case "extractFirmwareToFolder":
			payload := req.Payload.(map[string]interface{})
			zipFile := payload["zipFile"].(string)
			brand := payload["brand"].(string)
			model := payload["model"].(string)
			version := payload["version"].(string)
			androidVer := payload["androidVersion"].(string)
			binaryBit := payload["binaryBit"].(string)
			
			folder, files, errStr := ExtractFirmwareToFolder(zipFile, brand, model, version, androidVer, binaryBit)
			if errStr != "" {
				resp.Result = map[string]interface{}{"success": false, "error": errStr}
			} else {
				resp.Result = map[string]interface{}{
					"success": true,
					"folder":  folder,
					"files":   files,
					"count":   len(files),
				}
			}
		case "handleDroppedFirmware":
			payload := req.Payload.(map[string]interface{})
			filePath, _ := payload["filePath"].(string)
			resp.Result, resp.Error = handleDroppedFirmware(filePath)
		default:
			resp.Error = "unknown action"
		}

		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}

func runAdb(args ...string) (string, string, error) {
	adbPath, err := exec.LookPath("adb")
	if err != nil {
		return "", "", fmt.Errorf("ADB not found. Install Android Platform Tools and add to PATH.")
	}

	exec.Command(adbPath, "start-server").Run()

	cmd := exec.Command(adbPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
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
		return nil, "Invalid payload format"
	}

	model, _ := data["model"].(string)
	codename, _ := data["device"].(string)

	// Check multiple possible firmware locations
	searchDirs := []string{}
	
	// 1. Next to executable (dev mode)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		searchDirs = append(searchDirs, filepath.Join(exeDir, "firmware"))
		// Also check parent bin/ folder
		if strings.HasSuffix(exeDir, "bin") {
			searchDirs = append(searchDirs, filepath.Join(filepath.Dir(exeDir), "firmware"))
		}
	}
	
	// 2. AppData location (where installer saves)
	if appData := os.Getenv("APPDATA"); appData != "" {
		searchDirs = append(searchDirs, filepath.Join(appData, "AutoRoot", "firmware"))
	}
	
	// 3. LocalAppData fallback
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		searchDirs = append(searchDirs, filepath.Join(localAppData, "AutoRoot", "firmware"))
	}

	extensions := []string{".zip", ".tar.md5", ".img", ".tgz", ".7z", ".bin", ".payload.bin"}
	searchTerms := []string{model, strings.ToLower(model), codename, strings.ToLower(codename)}

	var foundFiles []string
	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		for _, term := range searchTerms {
			if term == "" { continue }
			for _, ext := range extensions {
				pattern := filepath.Join(dir, term+"*"+ext)
				matches, _ := filepath.Glob(pattern)
				for _, m := range matches {
					if !contains(foundFiles, filepath.Base(m)) {
						foundFiles = append(foundFiles, filepath.Base(m))
					}
				}
			}
		}
	}

	if len(foundFiles) > 0 {
		return map[string]interface{}{
			"available": true,
			"files":     foundFiles,
			"message":   "Firmware detected and ready for rooting.",
		}, ""
	}

	return map[string]interface{}{
		"available": false,
		"message":   "Firmware unavailable. Download during setup or from within the app.",
	}, ""
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}