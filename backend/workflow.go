package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Workflow actions exposed to the UI. Each one is a single JSON request/response
// action; the Electron main process whitelists them by name.

// StartSession begins (or resumes) a workflow for the connected phone.
func startSession(payload interface{}) (interface{}, string) {
	info, errStr := getDeviceInfo()
	if errStr != "" {
		return nil, errStr
	}
	device, ok := info.(map[string]interface{})
	if !ok {
		return nil, "unexpected device payload"
	}
	serial, _ := device["serial"].(string)
	model, _ := device["model"].(string)
	csc, _ := device["salesCode"].(string)
	binary, _ := device["binaryBit"].(string)

	// Resume when the persisted session belongs to this phone and has not
	// failed; otherwise start clean, because a failed run must be retried
	// from the beginning rather than continued.
	if existing := LoadSession(); existing != nil && existing.MatchesDevice(serial, model) {
		if existing.Stage == StageFailed {
			if err := ClearSession(); err != nil {
				return nil, fmt.Sprintf("cannot reset a failed session: %v", err)
			}
		} else {
			if err := existing.Advance(StageDeviceCheck); err != nil && !strings.Contains(err.Error(), "intermediate") {
				// A no-op re-entry is fine; a real error is not.
				if !strings.Contains(err.Error(), "failed state") {
					_ = existing.Advance(StageDeviceCheck)
				}
			}
			return map[string]interface{}{
				"resumed":   true,
				"stage":     existing.Stage,
				"session":   existing,
				"artifacts": existing.Artifacts,
			}, ""
		}
	}

	s := NewSession(serial, model, csc, binary)
	if err := s.Save(); err != nil {
		return nil, fmt.Sprintf("cannot persist session: %v", err)
	}
	if err := s.Advance(StageDeviceCheck); err != nil {
		return nil, err.Error()
	}
	return map[string]interface{}{
		"resumed": false,
		"stage":   s.Stage,
		"session": s,
	}, ""
}

// adoptFirmware validates a local firmware archive, records it on the session
// and returns the verdict. It is the only path by which a package becomes
// usable, so no downstream stage has to trust a filename.
func adoptFirmware(payload interface{}) (interface{}, string) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil, "invalid payload"
	}
	archive, _ := data["archivePath"].(string)
	archive = strings.TrimSpace(archive)
	if archive == "" {
		return nil, "archivePath is required"
	}

	s := LoadSession()
	if s == nil {
		return nil, "start a session before adopting firmware"
	}
	if s.Stage == StageFailed {
		return nil, fmt.Sprintf("the session failed (%s); start a new run", s.Failure)
	}

	// Extraction happens inside the device's own folder on the firmware volume,
	// never on the system drive.
	extractDir := filepath.Join(deviceFirmwareDir(s.Model), "extracted")
	validation := ValidateFirmwareArchive(archive, FirmwareProfile{
		Model:        s.Model,
		CSC:          s.CSC,
		DeviceBinary: s.DeviceBinary,
	}, extractDir)

	result := map[string]interface{}{
		"ok":        validation.OK,
		"errors":    validation.Errors,
		"warnings":  validation.Warnings,
		"modelCode": validation.ModelCode,
		"binary":    validation.PkgBinary,
		"slots":     map[string]interface{}{},
	}

	if !validation.OK {
		_ = s.Fail("firmware validation failed: " + strings.Join(validation.Errors, "; "))
		result["stage"] = StageFailed
		return result, ""
	}

	for slot, sf := range validation.Sizes {
		s.SetArtifact("slot-"+string(slot), sf.Path)
		result["slots"].(map[string]interface{})[string(slot)] = map[string]interface{}{
			"path": sf.Path, "file": sf.FileName, "size": sf.Size,
		}
	}
	s.SetArtifact("firmwareArchive", archive)
	s.SetProvenance("firmwareSource", "manual")
	s.SetProvenance("firmwarePath", archive)
	s.SetProvenance("modelCode", validation.ModelCode)
	s.SetProvenance("packageBinary", validation.PkgBinary)
	s.SetProvenance("validatedAt", time.Now().Format(time.RFC3339))

	if err := s.Advance(StageFirmwareFound); err != nil {
		return nil, err.Error()
	}
	if err := s.Advance(StageFirmwareValid); err != nil {
		return nil, err.Error()
	}
	result["stage"] = s.Stage
	result["session"] = s
	return result, ""
}

// fetchFirmware downloads a package straight to the firmware volume with
// resume, then validates it. dryRun stops before any bytes are transferred.
func fetchFirmware(payload interface{}) (interface{}, string) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil, "invalid payload"
	}
	url, _ := data["url"].(string)
	filename, _ := data["filename"].(string)
	dryRun, _ := data["dryRun"].(bool)

	s := LoadSession()
	if s == nil {
		return nil, "start a session before downloading firmware"
	}
	if strings.TrimSpace(url) == "" {
		return nil, "url is required"
	}
	if strings.TrimSpace(filename) == "" {
		filename = filepath.Base(url)
	}

	dest := filepath.Join(deviceFirmwareDir(s.Model), filename)

	// Pre-flight the destination so a full volume fails immediately instead of
	// after an hour of transfer.
	if !volFreeOK(deviceFirmwareDir(s.Model)) {
		return nil, fmt.Sprintf("not enough free space in %s; a Samsung package is about 5.81 GB", deviceFirmwareDir(s.Model))
	}

	req := DownloadRequest{
		URL:      url,
		DestPath: dest,
	}
	if sha, _ := data["sha256"].(string); sha != "" {
		req.ExpectedSHA256 = sha
	}
	if size, ok := data["size"].(float64); ok && size > 0 {
		req.ExpectedSize = int64(size)
	}

	if dryRun {
		partInfo := "(none)"
		if st, err := os.Stat(dest + partFileSuffix); err == nil {
			partInfo = formatSize(st.Size()) + " already downloaded, would resume from here"
		}
		return map[string]interface{}{
			"dryRun":      true,
			"ok":          true,
			"wouldFetch":  url,
			"destination": dest,
			"resumeState": partInfo,
			"freeSpace":   formatSize(volumeFreeBytes(deviceFirmwareDir(s.Model))),
			"message":     "Nothing was downloaded. Re-run with dryRun=false to start the transfer.",
		}, ""
	}

	res, err := Download(req, nil)
	if err != nil {
		return map[string]interface{}{
			"ok": false, "error": err.Error(),
			"partialPath": dest + partFileSuffix,
			"downloaded":  resBytes(res),
			"message":     "The transfer stopped. The partial file is kept, so running it again resumes instead of restarting.",
		}, ""
	}

	s.SetProvenance("firmwareSource", "samsung-fus")
	s.SetProvenance("firmwareURL", res.FinalURL)
	s.SetProvenance("downloadedAt", time.Now().Format(time.RFC3339))
	s.SetProvenance("downloadedSize", fmt.Sprintf("%d", res.Bytes))
	s.SetProvenance("downloadSHA256", res.SHA256)

	return map[string]interface{}{
		"ok":      true,
		"path":    res.Path,
		"bytes":   res.Bytes,
		"resumed": res.Resumed,
		"sha256":  res.SHA256,
		"verified": res.Verified,
		"warnings": res.Warnings,
		"duration": res.Duration.String(),
		"message":  "Downloaded. Run adoptFirmware to validate it before anything else.",
	}, ""
}

func resBytes(r *DownloadResult) int64 {
	if r == nil {
		return 0
	}
	return r.Bytes
}

// recordPatchedAP stores the Magisk-patched AP pulled from the phone. From
// here on the flash uses this file instead of the stock AP.
func recordPatchedAP(payload interface{}) (interface{}, string) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil, "invalid payload"
	}
	path, _ := data["path"].(string)
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, "path is required"
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || st.Size() == 0 {
		return nil, "the patched AP is missing or empty: " + path
	}

	s := LoadSession()
	if s == nil {
		return nil, "start a session first"
	}

	sum, err := FileSHA256(path)
	if err != nil {
		return nil, fmt.Sprintf("cannot hash the patched AP: %v", err)
	}
	// Guard against pulling the stock AP back by mistake: the patched file is
	// Magisk's output, so it must differ from the original.
	if stock := s.Artifact("slot-" + string(SlotAP)); stock != "" {
		if stockSum, err := FileSHA256(stock); err == nil && stockSum == sum {
			return nil, "the file pulled from the phone is byte-identical to the stock AP, so it was not patched; patch it in Magisk first"
		}
	}

	s.SetArtifact("patchedAP", path)
	s.SetProvenance("patchedAPSHA256", sum)
	if err := s.Advance(StagePatchedAP); err != nil {
		return nil, err.Error()
	}
	return map[string]interface{}{"ok": true, "path": path, "sha256": sum, "stage": s.Stage}, ""
}

// preflight runs every non-destructive gate.
func preflight(_ interface{}) (interface{}, string) {
	s := LoadSession()
	report := RunPreflight(s)
	return report, ""
}

// dryRun rehearses the entire workflow without flashing.
func dryRun(_ interface{}) (interface{}, string) {
	s := LoadSession()
	return RunDryRun(s), ""
}

// approveFlash records the explicit consent. It is deliberately a separate
// action: nothing else can grant it, and it is the only thing that unlocks
// the flash plan.
func approveFlash(payload interface{}) (interface{}, string) {
	data, _ := payload.(map[string]interface{})
	by, _ := data["by"].(string)
	if strings.TrimSpace(by) == "" {
		by = "operator"
	}

	s := LoadSession()
	if s == nil {
		return nil, "start a session first"
	}
	if s.Stage == StageFailed {
		return nil, fmt.Sprintf("the session failed (%s); start a new run", s.Failure)
	}
	if err := s.Advance(StagePreflightOK); err != nil {
		return nil, err.Error()
	}
	if err := s.GrantApproval(by, s.Artifact("patchedAP")); err != nil {
		return nil, err.Error()
	}
	if err := s.Advance(StageApprovedFlash); err != nil {
		return nil, err.Error()
	}
	return map[string]interface{}{"ok": true, "stage": s.Stage, "approvedBy": by}, ""
}

// flashPlan builds and returns the exact flash command without running it.
func flashPlan(payload interface{}) (interface{}, string) {
	data, _ := payload.(map[string]interface{})
	mode, _ := data["installMode"].(string)
	if mode == "" {
		mode = "initial-root"
	}

	s := LoadSession()
	if s == nil {
		return nil, "start a session first"
	}
	if err := s.RequireApproval(); err != nil {
		return nil, err.Error()
	}

	pkg, errStr := packageFromSession(s)
	if errStr != "" {
		return nil, errStr
	}
	plan, err := BuildFlashPlan(pkg, mode, s)
	if err != nil {
		return nil, err.Error()
	}
	return map[string]interface{}{
		"ok":           true,
		"command":      plan.CommandLine(),
		"engine":       plan.EnginePath,
		"engineVersion": plan.EngineVer,
		"installMode":  plan.InstallMode,
		"executed":     false,
		"message":      "This is the command that would run. Nothing has been flashed.",
	}, ""
}

// packageFromSession assembles the slot set for a flash, using the patched AP
// in place of the stock one.
func packageFromSession(s *Session) (map[Slot]string, string) {
	pkg := map[Slot]string{}
	patched := s.Artifact("patchedAP")
	if strings.TrimSpace(patched) == "" {
		return nil, "no patched AP has been recorded; patch the AP in Magisk and pull it first"
	}
	pkg[SlotAP] = patched

	for _, slot := range []Slot{SlotBL, SlotCP, SlotCSC} {
		path := s.Artifact("slot-" + string(slot))
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Sprintf("the %s file has not been produced", slot)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Sprintf("the %s file is missing: %s", slot, path)
		}
		pkg[slot] = path
	}
	return pkg, ""
}

// engineStatusAction exposes engine readiness to the UI.
func engineStatusAction(_ interface{}) (interface{}, string) {
	return engineStatus(), ""
}

// verifyRoot checks whether root is active, using a prompt-free command that
// cannot pop a dialog on the phone.
func verifyRoot(deviceID string) (interface{}, string) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, "deviceID is required"
	}
	out, stderr, err := runAdb("-s", deviceID, "shell", "su -c id")
	if err != nil && out == "" {
		return map[string]interface{}{"rooted": false, "message": "ADB query failed: " + stderr}, ""
	}
	rooted := strings.Contains(out, "uid=0")
	return map[string]interface{}{
		"rooted":  rooted,
		"raw":     out,
		"message": map[bool]string{true: "Root is active.", false: "Root is not active on this device."}[rooted],
	}, ""
}
