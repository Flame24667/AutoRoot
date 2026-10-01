package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempState points the state/cache directories at a scratch folder so a
// test never touches the developer's real session file.
func withTempState(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	prev := stateDirOverride
	stateDirOverride = dir
	t.Cleanup(func() { stateDirOverride = prev })
}

func TestSessionPersistsAcrossReload(t *testing.T) {
	withTempState(t)

	s := NewSession("TEST-DEVICE-001", "SM-A065F", "XID", "4")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if err := s.Advance(StageDeviceCheck); err != nil {
		t.Fatal(err)
	}
	if err := s.Advance(StageAdbAuthorized); err != nil {
		t.Fatal(err)
	}
	s.SetArtifact("firmware", `D:\AutoRoot\firmware\SM-A065F\fw.zip`)

	// A fresh load must see everything, which is what makes a resumed run
	// possible after the app is closed or the machine reboots.
	loaded := LoadSession()
	if loaded == nil {
		t.Fatal("expected the session to survive a reload")
	}
	if loaded.Stage != StageAdbAuthorized {
		t.Errorf("stage = %q, want %q", loaded.Stage, StageAdbAuthorized)
	}
	if loaded.Artifact("firmware") == "" {
		t.Error("the firmware artifact was lost across reload")
	}
	if !loaded.MatchesDevice("TEST-DEVICE-001", "SM-A065F") {
		t.Error("session should match its own device")
	}
}

func TestSessionRejectsSkippingStages(t *testing.T) {
	withTempState(t)
	s := NewSession("x", "SM-A065F", "XID", "4")

	// Jumping straight to flashing would bypass firmware validation entirely.
	err := s.Advance(StageFlashing)
	if err == nil {
		t.Fatal("expected a stage-skipping advance to be rejected")
	}
	if !strings.Contains(err.Error(), "intermediate stages") {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Stage == StageFlashing {
		t.Error("session must not skip ahead to the flashing stage")
	}
	if s.Stage != StageIdle {
		t.Errorf("a rejected advance must not move the stage, got %q", s.Stage)
	}
}

func TestSessionAdvancesForward(t *testing.T) {
	withTempState(t)
	s := NewSession("x", "SM-A065F", "XID", "4")

	for _, st := range []Stage{StageDeviceCheck, StageAdbAuthorized, StageFirmwareFound, StageFirmwareValid} {
		if err := s.Advance(st); err != nil {
			t.Fatal(err)
		}
		if s.Stage != st {
			t.Fatalf("stage = %q, want %q", s.Stage, st)
		}
	}
}

func TestReachedComparesStageOrder(t *testing.T) {
	withTempState(t)
	s := NewSession("x", "SM-A065F", "XID", "4")
	for _, st := range []Stage{StageDeviceCheck, StageAdbAuthorized, StageFirmwareFound, StageFirmwareValid} {
		_ = s.Advance(st)
	}
	if s.Stage != StageFirmwareValid {
		t.Fatalf("setup failed: stage = %q", s.Stage)
	}

	if !s.Reached(StageDeviceCheck) {
		t.Error("a session past firmware-valid has reached device-check")
	}
	if !s.Reached(StageFirmwareValid) {
		t.Error("a session at firmware-valid has not reached itself")
	}
	if s.Reached(StagePatchedAP) {
		t.Error("a session at firmware-valid must not have reached patched-ap")
	}
}

func TestFlashRequiresExplicitApproval(t *testing.T) {
	withTempState(t)
	s := NewSession("x", "SM-A065F", "XID", "4")

	if err := s.RequireApproval(); err == nil {
		t.Fatal("flashing must be refused without explicit approval")
	}

	if err := s.GrantApproval("user", "A065FXXS4AYE2"); err != nil {
		t.Fatal(err)
	}
	if err := s.RequireApproval(); err != nil {
		t.Fatalf("approval was not honoured: %v", err)
	}

	loaded := LoadSession()
	if loaded == nil || loaded.Approval == nil || !loaded.Approval.Granted {
		t.Error("approval did not survive a reload")
	}
}

func TestFailedSessionBlocksFurtherAdvance(t *testing.T) {
	withTempState(t)
	s := NewSession("x", "SM-A065F", "XID", "4")
	if err := s.Fail("md5 mismatch on AP"); err != nil {
		t.Fatal(err)
	}
	if s.Stage != StageFailed {
		t.Fatal("stage should be failed")
	}
	if err := s.Advance(StageDeviceCheck); err == nil {
		t.Error("a failed session must not advance again")
	}
	loaded := LoadSession()
	if loaded == nil || loaded.Failure != "md5 mismatch on AP" {
		t.Error("the failure reason did not survive a reload")
	}
}

func TestSessionRejectsForeignDevice(t *testing.T) {
	withTempState(t)
	s := NewSession("SERIAL_A", "SM-A065F", "XID", "4")
	_ = s.Save()

	if s.MatchesDevice("SERIAL_B", "SM-A065F") {
		t.Error("a session must not match a different serial")
	}
	if s.MatchesDevice("SERIAL_A", "SM-A055F") {
		t.Error("a session must not match a different model")
	}
}

func TestLoadSessionToleratesCorruptFile(t *testing.T) {
	withTempState(t)
	if err := os.WriteFile(stateFile(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A corrupt state file must not crash the backend; the app restarts idle.
	if s := LoadSession(); s != nil {
		t.Error("expected nil for an unparsable session file")
	}
}

func TestClearSessionRemovesState(t *testing.T) {
	withTempState(t)
	s := NewSession("x", "SM-A065F", "XID", "4")
	_ = s.Save()
	if err := ClearSession(); err != nil {
		t.Fatal(err)
	}
	if LoadSession() != nil {
		t.Error("state survived ClearSession")
	}
	// Clearing an absent session is not an error.
	if err := ClearSession(); err != nil {
		t.Errorf("ClearSession on an absent file should be a no-op, got %v", err)
	}
}

func TestSessionSaveIsAtomic(t *testing.T) {
	withTempState(t)
	s := NewSession("x", "SM-A065F", "XID", "4")
	_ = s.Save()

	// No .tmp file may survive a successful save.
	if _, err := os.Stat(stateFile() + ".tmp"); !os.IsNotExist(err) {
		t.Error("a temporary state file was left behind")
	}
	if strings.TrimSpace(stateFile()) == "" {
		t.Error("state file path must be absolute")
	}
	_ = filepath.Clean(stateFile())
}
