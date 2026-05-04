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
        return nil, "Invalid payload"
    }

    model, _ := data["model"].(string)
    codename, _ := data["device"].(string)

    // 🔑 1. GET FIRMWARE DIRECTORY FIRST
    fwDir := getFirmwareDirectory()
    if fwDir == "" {
        return map[string]interface{}{
            "available": false,
            "message":   "Firmware directory not found",
        }, ""
    }

    // 🔑 2. NOW CHECK FOR ANY FIRMWARE FILES (Fallback)
    files, _ := filepath.Glob(filepath.Join(fwDir, "*.zip"))
    imgFiles, _ := filepath.Glob(filepath.Join(fwDir, "*.img"))
    tarFiles, _ := filepath.Glob(filepath.Join(fwDir, "*.tar*"))

    if len(files) > 0 || len(imgFiles) > 0 || len(tarFiles) > 0 {
        allFiles := append(files, append(imgFiles, tarFiles...)...)
        return map[string]interface{}{
            "available": true,
            "message":   "Firmware files detected in folder.",
            "files":     allFiles,
        }, ""
    }

    // 3. Original model-specific search logic...
    searchPatterns := []string{
        filepath.Join(fwDir, "*"+model+"*"),
        filepath.Join(fwDir, "*"+codename+"*"),
    }

    var foundFiles []string
    for _, pattern := range searchPatterns {
        matches, err := filepath.Glob(pattern)
        if err == nil && len(matches) > 0 {
            foundFiles = append(foundFiles, matches...)
        }
    }

    if len(foundFiles) > 0 {
        return map[string]interface{}{
            "available": true,
            "files":     foundFiles,
            "count":     len(foundFiles),
        }, ""
    }

    return map[string]interface{}{
        "available": false,
        "message":   "No matching firmware found for this device",
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