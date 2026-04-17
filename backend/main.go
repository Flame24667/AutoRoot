package main

import (
	"bufio"
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
	fmt.Println("[Go] Backend started. Waiting for commands...")
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
		case "detectDevice":
			resp.Result, resp.Error = detectDevice()
		case "rootDevice":
			resp.Result, resp.Error = rootDevice(req.Payload)
		default:
			resp.Error = "unknown action"
		}

		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}

// --- LOGIC FUNCTIONS ---

func detectDevice() (interface{}, string) {
	// 1. Check if 'adb' is available on the system
	_, err := exec.LookPath("adb")
	if err != nil {
		// Fallback: Return mock data if ADB isn't installed so you can test the UI
		return map[string]interface{}{
			"brand":          "Mock Device (ADB not found)",
			"model":          "Test Phone",
			"androidVersion": "14.0",
			"rooted":         false,
		}, ""
	}

	// 2. Run 'adb devices' to see if a phone is connected
	cmd := exec.Command("adb", "devices")
	out, err := cmd.Output()
	if err != nil {
		return nil, "ADB error: " + err.Error()
	}

	// 3. Parse output (simplified)
	// ADB output looks like:
	// List of devices attached
	// 1234567890	device
	lines := strings.Split(string(out), "\n")
	if len(lines) < 3 || !strings.Contains(lines[2], "device") {
		return nil, "No device connected"
	}

	// 4. Get device details
	deviceID := strings.Fields(lines[2])[0]
	
	model, _ := exec.Command("adb", "-s", deviceID, "shell", "getprop", "ro.product.model").Output()
	brand, _ := exec.Command("adb", "-s", deviceID, "shell", "getprop", "ro.product.brand").Output()
	version, _ := exec.Command("adb", "-s", deviceID, "shell", "getprop", "ro.build.version.release").Output()
	
	rootCheck, _ := exec.Command("adb", "-s", deviceID, "shell", "su", "-c", "id").Output()
	isRooted := strings.Contains(string(rootCheck), "uid=0")

	return map[string]interface{}{
		"brand":          strings.TrimSpace(string(brand)),
		"model":          strings.TrimSpace(string(model)),
		"androidVersion": strings.TrimSpace(string(version)),
		"rooted":         isRooted,
	}, ""
}

func rootDevice(payload interface{}) (interface{}, string) {
	// TODO: Insert actual rooting logic here
	// Example: exec.Command("adb", "shell", "su", "-c", "mount -o remount,rw /system").Run()
	
	return map[string]interface{}{
		"message": "Root process simulated successfully.",
	}, ""
}