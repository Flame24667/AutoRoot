package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
			resp.Result, resp.Error = getDeviceInfo()
		case "rootDevice":
			resp.Result, resp.Error = rootDevice()
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
	// 1. Try exact match first
	if name, ok := deviceNames[model]; ok {
		return name
	}
	
	// 2. Try prefix matching (e.g., "ASUS_AI2401_D" matches "ASUS_AI2401")
	for code, name := range deviceNames {
		if strings.HasPrefix(model, code) {
			return name
		}
	}
	
	// 3. Try without "SM-" prefix for Samsung
	if brand == "samsung" && strings.HasPrefix(model, "SM-") {
		baseModel := strings.TrimPrefix(model, "SM-")
		if name, ok := deviceNames[baseModel]; ok {
			return name
		}
		// Also try prefix matching on base model
		for code, name := range deviceNames {
			if strings.HasPrefix(baseModel, code) {
				return name
			}
		}
	}
	
	// 4. Return original model if no match found
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
	version, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.build.version.release")
	rootOut, _, _ := runAdb("-s", deviceID, "shell", "su", "-c", "id")
	typeVal, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.build.characteristics")
	osVersion, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.build.version.release")
	binary, _, _ := runAdb("-s", deviceID, "shell", "getprop", "ro.build.version.incremental")
	ram, _, _ := runAdb("-s", deviceID, "shell", "cat", "/proc/meminfo")
	cpu, _, _ := runAdb("-s", deviceID, "shell", "cat", "/proc/cpuinfo")
	kernel, _, _ := runAdb("-s", deviceID, "shell", "uname", "-a")

	// Parse RAM (MemTotal)
	ramTotal := ""
	ramLines := strings.Split(ram, "\n")
	for _, line := range ramLines {
		if strings.HasPrefix(line, "MemTotal:") {
			ramTotal = strings.TrimSpace(line)
			break
		}
	}

	// Parse CPU (model name)
	cpuModel := ""
	cpuLines := strings.Split(cpu, "\n")
	for _, line := range cpuLines {
		if strings.HasPrefix(line, "Hardware") || strings.HasPrefix(line, "model name") {
			cpuModel = strings.TrimSpace(line)
			break
		}
	}

	return map[string]interface{}{
		"brand":          strings.TrimSpace(brand),
		"model":          strings.TrimSpace(model),
		"phoneType":      strings.TrimSpace(typeVal),
		"androidVersion": strings.TrimSpace(version),
		"osVersion":      strings.TrimSpace(osVersion),
		"binary":         strings.TrimSpace(binary),
		"ram":            ramTotal,
		"cpu":            cpuModel,
		"kernelVersion":  strings.TrimSpace(kernel),
		"rooted":         strings.Contains(rootOut, "uid=0"),
	}, ""
}

func rootDevice() (interface{}, string) {
	return map[string]interface{}{"message": "Root process initiated. Monitor device screen."}, ""
}
