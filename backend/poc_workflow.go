package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func rootIdentity(out string) bool {
	return regexp.MustCompile(`(^|\s)uid=0\(root\)(\s|$)`).MatchString(out)
}

func validateNewRun(s *Session, device map[string]interface{}, confirmed bool, expectedSerial string, suAbsent bool) error {
	serial, _ := device["serial"].(string)
	model, _ := device["model"].(string)
	rooted, _ := device["rooted"].(bool)
	locked, _ := device["bootloaderLocked"].(bool)
	if !confirmed || expectedSerial == "" || serial != expectedSerial {
		return fmt.Errorf("explicit new-run confirmation for the connected serial is required")
	}
	if model != "SM-A065F" || locked || rooted || !suAbsent {
		return fmt.Errorf("new initial-root run requires SM-A065F, unlocked bootloader and no su executable; restore full stock first")
	}
	if s != nil && (!s.MatchesDevice(serial, model) || s.Stage == StageFlashing || s.Stage == StageDownloadMode) {
		return fmt.Errorf("saved run belongs to another phone or has an unresolved flash stage; inspect it instead of resetting")
	}
	return nil
}

func archiveSession(s *Session) (string, error) {
	if s == nil {
		return "", nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	dir := filepath.Join(writableStateDir(), "history")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "session-*.json")
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return path, nil
}

// This clears local approval/artifacts only after an operator-confirmed stock
// restore. It never unroots, wipes, unlocks, reboots or flashes the phone.
func beginNewRun(payload interface{}) (interface{}, string) {
	p, _ := payload.(map[string]interface{})
	confirmed, _ := p["confirmNewRun"].(bool)
	serial, _ := p["serial"].(string)
	if !confirmed || strings.TrimSpace(serial) == "" {
		return nil, "explicit new-run confirmation and serial required"
	}
	info, errText := getDeviceInfo()
	if errText != "" {
		return nil, errText
	}
	device, ok := info.(map[string]interface{})
	if !ok {
		return nil, "unexpected device info"
	}
	// A denied root request must not be mistaken for a stock device.
	check, _, err := runAdb("-s", serial, "shell", "command -v su; echo AUTOROOT_SU_CHECK:$?")
	suAbsent := err == nil && strings.TrimSpace(check) == "AUTOROOT_SU_CHECK:1"
	old := LoadSession()
	if err = validateNewRun(old, device, confirmed, serial, suAbsent); err != nil {
		return nil, err.Error()
	}
	archive, err := archiveSession(old)
	if err != nil {
		return nil, "cannot archive old evidence: " + err.Error()
	}
	model, _ := device["model"].(string)
	csc, _ := device["salesCode"].(string)
	binary, _ := device["binaryBit"].(string)
	s := NewSession(serial, model, csc, binary)
	s.Provenance["deviceBuild"] = fmt.Sprint(device["buildVersion"])
	s.Provenance["deviceCSCBuild"] = fmt.Sprint(device["cscBuild"])
	if err = s.Save(); err != nil {
		return nil, err.Error()
	}
	for _, stage := range []Stage{StageDeviceCheck, StageAdbAuthorized} {
		if err = s.Advance(stage); err != nil {
			return nil, err.Error()
		}
	}
	return map[string]interface{}{"ok": true, "session": s, "archivedSession": archive, "message": "New local run started; old evidence archived. No firmware/phone data was changed."}, ""
}
