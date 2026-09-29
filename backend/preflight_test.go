package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func blockerFor(report *PreflightReport, id string) *PreflightCheck {
	for i := range report.Checks {
		if report.Checks[i].ID == id {
			return &report.Checks[i]
		}
	}
	return nil
}

func TestPreflightBlocksWithoutDevice(t *testing.T) {
	withTempState(t)
	// No session and, in the test environment, no guaranteed device. Whatever
	// happens, an unavailable phone must block the flash.
	report := RunPreflight(nil)
	if report.OK {
		t.Skip("a device is connected in this environment; the no-device path cannot be asserted")
	}
	if len(report.Blockers) == 0 {
		t.Fatal("a blocked preflight must explain why")
	}
	if !strings.Contains(report.Summary, "blocked") {
		t.Errorf("summary should say the flash is blocked, got %q", report.Summary)
	}
}

func TestPreflightReportStructure(t *testing.T) {
	withTempState(t)
	report := RunPreflight(nil)

	if report.Summary == "" {
		t.Error("every preflight report needs a summary")
	}
	for _, c := range report.Checks {
		if c.ID == "" || c.Label == "" {
			t.Errorf("check %+v is missing an id or label", c)
		}
		if !c.Required {
			t.Errorf("check %q is marked optional; every gate here blocks flashing", c.ID)
		}
	}
}

func TestPreflightNeverPassesWithoutApproval(t *testing.T) {
	withTempState(t)
	// Even with a fully populated session, the flash itself stays locked
	// behind RequireApproval, which BuildFlashPlan enforces.
	s := NewSession("R9RY100N48L", "SM-A065F", "XID", "4")
	if err := s.RequireApproval(); err == nil {
		t.Fatal("a fresh session must not be approved")
	}
	_ = s.Advance(StageDeviceCheck)
	_ = s.Advance(StageAdbAuthorized)
	_ = s.Advance(StageFirmwareFound)
	_ = s.Advance(StageFirmwareValid)
	if err := s.RequireApproval(); err == nil {
		t.Fatal("reaching firmware-valid must not imply approval to flash")
	}
}

func TestDryRunNeverFlashes(t *testing.T) {
	withTempState(t)
	useFakeEngine(t)

	s := NewSession("R9RY100N48L", "SM-A065F", "XID", "4")
	_ = s.Save()

	// A dry run with nothing prepared must fail cleanly and never claim success.
	res := RunDryRun(s)
	if res.Summary == "" {
		t.Error("a dry run must produce a summary")
	}
	for _, step := range res.Steps {
		if strings.Contains(strings.ToLower(step.Name), "flash") &&
			strings.Contains(strings.ToLower(step.Detail), "executed") {
			t.Errorf("dry run reported an executed flash: %+v", step)
		}
	}
}

func TestPackageFromSessionRequiresPatchedAP(t *testing.T) {
	withTempState(t)
	s := NewSession("R9RY100N48L", "SM-A065F", "XID", "4")
	_ = s.Save()

	if _, errStr := packageFromSession(s); errStr == "" {
		t.Fatal("a session with no patched AP must not produce a flashable package")
	} else if !strings.Contains(errStr, "patched AP") {
		t.Fatalf("error should name the missing patched AP, got %q", errStr)
	}
}

func TestPackageFromSessionReplacesStockAP(t *testing.T) {
	withTempState(t)
	dir := t.TempDir()
	stock := writeFakeSlotFiles(t, dir, "CSC_A065FXXS4AYE2_A065FOLE4AYE2_x.tar.md5")
	patched := filepath.Join(dir, "magisk_patched_A065FXXS4AYE2.tar")

	s := NewSession("R9RY100N48L", "SM-A065F", "XID", "4")
	for slot, path := range stock {
		s.SetArtifact("slot-"+string(slot), path)
	}
	s.SetArtifact("patchedAP", patched)

	pkg, errStr := packageFromSession(s)
	if errStr != "" {
		t.Fatalf("unexpected error: %s", errStr)
	}
	if pkg[SlotAP] != patched {
		t.Errorf("AP should be the patched file, got %q", pkg[SlotAP])
	}
	if pkg[SlotBL] == "" || pkg[SlotCP] == "" || pkg[SlotCSC] == "" {
		t.Error("the other slots should be carried through unchanged")
	}
}
