package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// PreflightCheck is one condition that must hold before flashing.
type PreflightCheck struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Passed   bool   `json:"passed"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

// PreflightReport is the full verdict: a list of checks plus whether flashing
// may be attempted. Nothing here touches the phone's flashable partitions.
type PreflightReport struct {
	OK       bool             `json:"ok"`
	Checks   []PreflightCheck `json:"checks"`
	Blockers []string         `json:"blockers,omitempty"`
	Warnings []string         `json:"warnings,omitempty"`
	Summary  string           `json:"summary"`
}

// blocked reports whether a required check failed.
func (r *PreflightReport) blocked() bool {
	for _, c := range r.Checks {
		if c.Required && !c.Passed {
			return true
		}
	}
	return false
}

// RunPreflight assembles every gate that must pass before a flash. It is
// purely read-only: it inspects local files and asks ADB for device state, and
// it never writes to the phone.
//
// The order mirrors the target flow, so a failure report reads top-to-bottom as
// an explanation of what still has to happen.
func RunPreflight(session *Session) *PreflightReport {
	report := &PreflightReport{}
	add := func(c PreflightCheck) { report.Checks = append(report.Checks, c) }

	// 1. ADB must be usable, otherwise nothing else can be trusted.
	info, errStr := getDeviceInfo()
	if errStr != "" {
		add(PreflightCheck{
			ID: "adb", Label: "ADB is authorized and the phone is connected",
			Passed: false, Required: true, Detail: errStr,
		})
		// Without a device there is nothing meaningful left to check; stop so
		// the report is not padded with cascading failures.
		return finalizePreflight(report)
	}
	device, _ := info.(map[string]interface{})
	add(PreflightCheck{ID: "adb", Label: "ADB is authorized and the phone is connected", Passed: true, Required: true,
		Detail: fmt.Sprintf("%v", device["model"])})

	// 2. The session must belong to this phone, so a run started for another
	// device can never be continued here.
	serial, _ := device["serial"].(string)
	model, _ := device["model"].(string)
	battery, _, batteryErr := runAdb("-s", serial, "shell", "dumpsys", "battery")
	level, levelErr := batteryLevel(battery)
	add(PreflightCheck{ID: "battery", Label: "Battery meets AutoRoot's conservative 60% threshold", Passed: batteryErr == nil && levelErr == nil && level >= 60, Required: true, Detail: fmt.Sprintf("level=%d; unreadable battery status blocks flashing", level)})
	if session == nil {
		add(PreflightCheck{ID: "session", Label: "A workflow session exists for this phone",
			Passed: false, Required: true, Detail: "no session has been started"})
	} else if !session.MatchesDevice(serial, model) {
		add(PreflightCheck{ID: "session", Label: "A workflow session exists for this phone",
			Passed: false, Required: true,
			Detail: fmt.Sprintf("session is for %s/%s but the connected phone is %s/%s",
				session.Serial, session.Model, serial, model)})
	} else {
		add(PreflightCheck{ID: "session", Label: "A workflow session exists for this phone",
			Passed: true, Required: true, Detail: "session " + string(session.Stage)})
		build, _ := device["buildVersion"].(string)
		cscBuild, _ := device["cscBuild"].(string)
		csc, _ := device["salesCode"].(string)
		add(PreflightCheck{ID: "current-build", Label: "Current PDA, OMC and CSC still match validated session", Passed: build != "" && build == session.Provenance["deviceBuild"] && cscBuild != "" && cscBuild == session.Provenance["deviceCSCBuild"] && csc == session.CSC, Required: true})
	}

	// 3. Bootloader must be unlocked. A locked bootloader cannot accept
	// Magisk-patched firmware at all, and re-locking is never attempted.
	locked, _ := device["bootloaderLocked"].(bool)
	vb, _ := device["verifiedBootState"].(string)
	if locked {
		add(PreflightCheck{ID: "bootloader", Label: "Bootloader is unlocked",
			Passed: false, Required: true,
			Detail: fmt.Sprintf("ro.boot.flash.locked is non-zero (verified boot: %s). Unlocking is a manual, device-specific operation and AutoRoot will not attempt it.", vb)})
	} else {
		add(PreflightCheck{ID: "bootloader", Label: "Bootloader is unlocked",
			Passed: true, Required: true, Detail: "verified boot: " + vb})
	}

	// 4. The phone must not already be rooted for a first-time install; a
	// rooted phone needs the HOME_CSC path instead.
	rooted, _ := device["rooted"].(bool)
	if rooted {
		add(PreflightCheck{ID: "root-state", Label: "Device is not already rooted",
			Passed: false, Required: true,
			Detail: "root is already active; an already-rooted phone must use HOME_CSC via the rooted-update path"})
	} else {
		add(PreflightCheck{ID: "root-state", Label: "Device is not already rooted", Passed: true, Required: true})
	}

	// 5. A validated firmware package must already exist on disk.
	if session == nil {
		return finalizePreflight(report)
	}
	add(PreflightCheck{ID: "validator-version", Label: "Real Samsung embedded-MD5 validator was used", Passed: session.Provenance["validatorVersion"] == "embedded-md5-v1", Required: true})
	add(PreflightCheck{ID: "validated-stage", Label: "Firmware and patch completed validation", Passed: session.Reached(StagePatchedAP), Required: true})
	add(PreflightCheck{ID: "csc-proof", Label: "Installed OMC build matches validated CSC", Passed: session.Provenance["deviceCSCBuild"] != "" && strings.Contains(upperBase(session.Artifact("slot-CSC")), strings.ToUpper(session.Provenance["deviceCSCBuild"])+"_"), Required: true})
	archive := session.Artifact("firmwareArchive")
	if strings.TrimSpace(archive) == "" {
		add(PreflightCheck{ID: "firmware", Label: "A validated firmware package is present",
			Passed: false, Required: true, Detail: "no firmware has been downloaded or validated yet"})
	} else if st, err := os.Stat(archive); err != nil || st.IsDir() {
		add(PreflightCheck{ID: "firmware", Label: "A validated firmware package is present",
			Passed: false, Required: true, Detail: "the recorded firmware file is missing: " + archive})
	} else {
		add(PreflightCheck{ID: "firmware", Label: "A validated firmware package is present",
			Passed: true, Required: true, Detail: fmt.Sprintf("%s (%s)", filepath.Base(archive), formatSize(st.Size()))})
	}

	// 6. Each slot the flash will use must still exist and still be readable.
	for _, slot := range RequiredSlots {
		path := session.Artifact("slot-" + string(slot))
		label := "The " + string(slot) + " file is ready"
		if path == "" {
			add(PreflightCheck{ID: "slot-" + string(slot), Label: label, Passed: false, Required: true,
				Detail: "not produced yet"})
			continue
		}
		st, err := os.Stat(path)
		if err != nil || st.IsDir() || st.Size() == 0 {
			add(PreflightCheck{ID: "slot-" + string(slot), Label: label, Passed: false, Required: true,
				Detail: "missing or empty: " + path})
			continue
		}
		add(PreflightCheck{ID: "slot-" + string(slot), Label: label, Passed: true, Required: true,
			Detail: formatSize(st.Size())})
		if slot != SlotAP {
			e := verifyEmbeddedMD5(path)
			detail := "embedded MD5 verified again before flash"
			if e != nil {
				detail = e.Error()
			}
			add(PreflightCheck{ID: "integrity-" + string(slot), Label: string(slot) + " embedded MD5 still matches", Passed: e == nil, Required: true, Detail: detail})
		}
	}

	// 7. The AP that will actually be flashed must be the Magisk-patched one.
	// Flashing a stock AP would silently discard the root install.
	patched := session.Artifact("patchedAP")
	if strings.TrimSpace(patched) == "" {
		add(PreflightCheck{ID: "patched", Label: "The AP is Magisk-patched",
			Passed: false, Required: true,
			Detail: "no patched AP has been pulled from the phone; patching the AP in Magisk is a manual on-device step"})
	} else if st, err := os.Stat(patched); err != nil || st.IsDir() || st.Size() == 0 {
		add(PreflightCheck{ID: "patched", Label: "The AP is Magisk-patched",
			Passed: false, Required: true, Detail: "the patched AP is missing: " + patched})
	} else {
		add(PreflightCheck{ID: "patched", Label: "The AP is Magisk-patched",
			Passed: true, Required: true, Detail: formatSize(st.Size())})
		sum, e := FileSHA256(patched)
		add(PreflightCheck{ID: "patch-integrity", Label: "Patched AP still matches its recorded SHA-256", Passed: e == nil && len(session.Provenance["patchedAPSHA256"]) == 64 && sum == session.Provenance["patchedAPSHA256"], Required: true})
	}

	// 8. The flashing engine must be installed and checksum-verified.
	status := engineStatus()
	engineOK, _ := status["available"].(bool)
	engineDetail, _ := status["reason"].(string)
	if engineDetail == "" {
		engineDetail = fmt.Sprintf("%v", status["path"])
	}
	add(PreflightCheck{ID: "engine", Label: "Flashing engine is installed and verified",
		Passed: engineOK, Required: true, Detail: engineDetail})

	// 9. The destination volume must still have room; a flash writes gigabytes.
	destDir := deviceFirmwareDir(model)
	if volFreeOK(destDir) {
		add(PreflightCheck{ID: "space", Label: "Firmware volume has free space",
			Passed: true, Required: true, Detail: destDir})
	} else {
		add(PreflightCheck{ID: "space", Label: "Firmware volume has free space",
			Passed: false, Required: true, Detail: destDir + " is below the required headroom"})
	}

	// 10. Magisk itself must be installed for the patch step to have happened.
	if magiskInstalled(serial) {
		add(PreflightCheck{ID: "magisk", Label: "Magisk is installed on the phone",
			Passed: true, Required: true})
	} else {
		add(PreflightCheck{ID: "magisk", Label: "Magisk is installed on the phone",
			Passed: false, Required: true, Detail: "com.topjohnwu.magisk is not installed"})
	}

	return finalizePreflight(report)
}

func batteryLevel(data string) (int, error) {
	m := regexp.MustCompile(`(?m)^\s*level:\s*([0-9]+)\s*$`).FindStringSubmatch(data)
	if len(m) != 2 {
		return 0, fmt.Errorf("battery level missing")
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 0 || n > 100 {
		return 0, fmt.Errorf("invalid battery level")
	}
	return n, nil
}

func finalizePreflight(report *PreflightReport) *PreflightReport {
	for _, c := range report.Checks {
		switch {
		case c.Required && !c.Passed:
			report.Blockers = append(report.Blockers, c.Label+": "+c.Detail)
		case !c.Required && !c.Passed:
			report.Warnings = append(report.Warnings, c.Label+": "+c.Detail)
		}
	}
	report.OK = !report.blocked()
	if report.OK {
		report.Summary = fmt.Sprintf("All %d preflight checks passed. The device is ready to be flashed, but flashing still requires explicit approval.",
			len(report.Checks))
	} else {
		report.Summary = fmt.Sprintf("%d of %d preflight checks failed. Flashing is blocked.",
			len(report.Blockers), len(report.Checks))
	}
	return report
}

// magiskInstalled reports whether the Magisk package is present on the device.
func magiskInstalled(deviceID string) bool {
	out, _, err := runAdb("-s", deviceID, "shell", "pm", "path", "com.topjohnwu.magisk")
	if err != nil {
		return false
	}
	return strings.Contains(out, "package:")
}

// DryRunResult is the output of an end-to-end rehearsal. It performs every
// non-destructive step and reports exactly what a real run would do, without
// rebooting the phone or writing a single flashable byte.
type DryRunResult struct {
	OK        bool             `json:"ok"`
	Steps     []DryRunStep     `json:"steps"`
	Preflight *PreflightReport `json:"preflight"`
	FlashCmd  string           `json:"flashCommand,omitempty"`
	Summary   string           `json:"summary"`
}

// DryRunStep is one rehearsed action.
type DryRunStep struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// RunDryRun walks the whole workflow up to (but never including) the flash.
// It is the fastest way to find out what is still missing, and it is safe to
// run at any time because nothing it does can brick the phone.
func RunDryRun(session *Session) *DryRunResult {
	res := &DryRunResult{OK: true}
	step := func(name string, ok bool, format string, args ...interface{}) {
		res.Steps = append(res.Steps, DryRunStep{
			Name: name, OK: ok, Detail: fmt.Sprintf(format, args...),
		})
		if !ok {
			res.OK = false
		}
	}

	// Device detection.
	info, errStr := getDeviceInfo()
	if errStr != "" {
		step("detect device", false, "%s", errStr)
		res.Preflight = RunPreflight(session)
		res.Summary = "Stopped at device detection."
		return res
	}
	device, _ := info.(map[string]interface{})
	serial, _ := device["serial"].(string)
	model, _ := device["model"].(string)
	csc, _ := device["salesCode"].(string)
	binary, _ := device["binaryBit"].(string)
	step("detect device", true, "%v %v, CSC %s, bootloader binary %s", device["brand"], model, csc, binary)

	// Session continuity.
	if session != nil && session.MatchesDevice(serial, model) {
		step("resume session", true, "continuing from stage %q", session.Stage)
	} else {
		step("resume session", false, "no session for %s; start a run before flashing", serial)
	}

	// Engine.
	status := engineStatus()
	if ok, _ := status["available"].(bool); ok {
		step("flashing engine", true, "samloader %v verified at %v", status["version"], status["path"])
	} else {
		step("flashing engine", false, "%v", status["reason"])
	}

	// Firmware.
	if session != nil {
		if archive := session.Artifact("firmwareArchive"); archive != "" {
			if st, err := os.Stat(archive); err == nil {
				step("firmware package", true, "%s (%s)", filepath.Base(archive), formatSize(st.Size()))
			} else {
				step("firmware package", false, "recorded file is missing: %s", archive)
			}
		} else {
			step("firmware package", false, "no firmware has been downloaded yet")
		}
	}

	// Patched AP.
	if session != nil {
		if patched := session.Artifact("patchedAP"); patched != "" {
			if st, err := os.Stat(patched); err == nil {
				step("patched AP", true, "%s (%s)", filepath.Base(patched), formatSize(st.Size()))
			} else {
				step("patched AP", false, "recorded patched AP is missing: %s", patched)
			}
		} else {
			step("patched AP", false, "the AP has not been patched in Magisk yet")
		}
	}

	// Build the flash command if everything is in place, so the operator can
	// see the exact invocation before approving it.
	if session != nil {
		pkg := map[Slot]string{}
		complete := true
		for _, slot := range RequiredSlots {
			key := "slot-" + string(slot)
			if slot == SlotAP {
				// The patched AP replaces the stock one.
				if p := session.Artifact("patchedAP"); p != "" {
					pkg[slot] = p
					continue
				}
				complete = false
				break
			}
			if p := session.Artifact(key); p != "" {
				pkg[slot] = p
			} else {
				complete = false
			}
		}
		if complete {
			// Dry run passes a throwaway approval so the plan can be rendered;
			// nothing is executed and no real approval is recorded.
			probe := &Session{Approval: &FlashApproval{Granted: true, GrantedBy: "dry-run"}}
			plan, err := BuildFlashPlan(pkg, "initial-root", probe)
			if err != nil {
				step("flash plan", false, "%v", err)
			} else {
				res.FlashCmd = plan.CommandLine()
				step("flash plan", true, "would run: %s", res.FlashCmd)
			}
		} else {
			step("flash plan", false, "firmware set is incomplete")
		}
	}

	res.Preflight = RunPreflight(session)
	if res.OK && res.Preflight.OK {
		res.Summary = "Dry run passed. The only remaining step is flashing, which requires your explicit approval."
	} else {
		res.Summary = "Dry run found gaps. See the steps and preflight report above."
	}
	return res
}
