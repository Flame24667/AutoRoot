package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeSlotFiles(t *testing.T, dir string, cscName string) map[Slot]string {
	t.Helper()
	files := map[Slot]string{}
	names := map[Slot]string{
		SlotAP:  "AP_A065FXXS4AYE2_A065FOLE4AYE2_20240401.tar.md5",
		SlotBL:  "BL_A065FXXS4AYE2_A065FXXS4AYE1_20240401.tar.md5",
		SlotCP:  "CP_A065FXXS4AYE2_A065FXXS4AYE1_20240401.tar.md5",
		SlotCSC: cscName,
	}
	for slot, name := range names {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		files[slot] = p
	}
	return files
}

// useFakeEngine points the engine resolver at a stub file so plan building can
// be tested without a real samloader binary. The pin check is bypassed via the
// test-only pin, never by skipping verification in production code.
func useFakeEngine(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	fake := filepath.Join(dir, "samloader.exe")
	if err := os.WriteFile(fake, []byte("MZ fake engine"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTOROOT_SAMLOADER", fake)

	sum, err := FileSHA256(fake)
	if err != nil {
		t.Fatal(err)
	}
	prev, had := samloaderPins[samloaderVersion]
	samloaderPins[samloaderVersion] = sum
	t.Cleanup(func() {
		if had {
			samloaderPins[samloaderVersion] = prev
		} else {
			delete(samloaderPins, samloaderVersion)
		}
	})
}

func approvedSession(t *testing.T) *Session {
	t.Helper()
	withTempState(t)
	s := NewSession("R9RY100N48L", "SM-A065F", "XID", "4")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantApproval("test", "A065FXXS4AYE2"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEngineRefusedWhenNoPinRegistered(t *testing.T) {
	withTempState(t)
	dir := t.TempDir()
	fake := filepath.Join(dir, "samloader.exe")
	if err := os.WriteFile(fake, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTOROOT_SAMLOADER", fake)

	// Remove the pin: an unverified engine must never be executed.
	prev, had := samloaderPins[samloaderVersion]
	delete(samloaderPins, samloaderVersion)
	t.Cleanup(func() {
		if had {
			samloaderPins[samloaderVersion] = prev
		}
	})

	if _, err := resolveSamloader(); err == nil {
		t.Fatal("expected an unpinned engine to be refused")
	} else if !strings.Contains(err.Error(), "pinned") {
		t.Fatalf("error should mention the pin, got %v", err)
	}
}

func TestEngineRefusedOnChecksumMismatch(t *testing.T) {
	withTempState(t)
	dir := t.TempDir()
	fake := filepath.Join(dir, "samloader.exe")
	if err := os.WriteFile(fake, []byte("MZ tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTOROOT_SAMLOADER", fake)

	prev, had := samloaderPins[samloaderVersion]
	samloaderPins[samloaderVersion] = strings.Repeat("a", 64)
	t.Cleanup(func() {
		if had {
			samloaderPins[samloaderVersion] = prev
		} else {
			delete(samloaderPins, samloaderVersion)
		}
	})

	_, err := resolveSamloader()
	if err == nil {
		t.Fatal("expected a checksum mismatch to refuse the engine")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error should name the checksum, got %v", err)
	}
}

func TestBuildFlashPlanRefusesWithoutApproval(t *testing.T) {
	useFakeEngine(t)
	withTempState(t)
	files := writeFakeSlotFiles(t, t.TempDir(), "CSC_A065FXXS4AYE2_A065FOLE4AYE2_20240401.tar.md5")

	s := NewSession("R9RY100N48L", "SM-A065F", "XID", "4")
	_ = s.Save()

	_, err := BuildFlashPlan(files, "initial-root", s)
	if err == nil {
		t.Fatal("flashing must be impossible without explicit approval")
	}
	if !strings.Contains(err.Error(), "explicit approval") {
		t.Fatalf("error should mention approval, got %v", err)
	}
}

func TestBuildFlashPlanRejectsHomeCSCForInitialRoot(t *testing.T) {
	useFakeEngine(t)
	files := writeFakeSlotFiles(t, t.TempDir(), "HOME_CSC_A065FXXS4AYE2_A065FOLE4AYE2_20240401.tar.md5")

	_, err := BuildFlashPlan(files, "initial-root", approvedSession(t))
	if err == nil {
		t.Fatal("an initial root install must not accept HOME_CSC")
	}
	if !strings.Contains(err.Error(), "standard CSC") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildFlashPlanRejectsStandardCSCForRootedUpdate(t *testing.T) {
	useFakeEngine(t)
	files := writeFakeSlotFiles(t, t.TempDir(), "CSC_A065FXXS4AYE2_A065FOLE4AYE2_20240401.tar.md5")

	_, err := BuildFlashPlan(files, "rooted-update", approvedSession(t))
	if err == nil {
		t.Fatal("a rooted update must not accept a wiping CSC")
	}
	if !strings.Contains(err.Error(), "HOME_CSC") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildFlashPlanProducesExpectedArgs(t *testing.T) {
	useFakeEngine(t)
	files := writeFakeSlotFiles(t, t.TempDir(), "CSC_A065FXXS4AYE2_A065FOLE4AYE2_20240401.tar.md5")

	plan, err := BuildFlashPlan(files, "initial-root", approvedSession(t))
	if err != nil {
		t.Fatalf("plan should build: %v", err)
	}

	cmd := plan.CommandLine()
	for _, want := range []string{"flash", "--BL", "--AP", "--CP", "--CSC"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command line missing %q: %s", want, cmd)
		}
	}
	// A boolean switch must never be given an "=false" value; samloader would
	// reject it as a usage error at flash time.
	if strings.Contains(cmd, "--no-reboot=") {
		t.Errorf("command line contains an invalid boolean form: %s", cmd)
	}
}

func TestBuildFlashPlanRejectsMissingSlot(t *testing.T) {
	useFakeEngine(t)
	files := writeFakeSlotFiles(t, t.TempDir(), "CSC_A065FXXS4AYE2_A065FOLE4AYE2_20240401.tar.md5")
	delete(files, SlotCP)

	_, err := BuildFlashPlan(files, "initial-root", approvedSession(t))
	if err == nil {
		t.Fatal("expected a missing CP to be rejected")
	}
	if !strings.Contains(err.Error(), "CP") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildFlashPlanRejectsUnknownInstallMode(t *testing.T) {
	useFakeEngine(t)
	files := writeFakeSlotFiles(t, t.TempDir(), "CSC_A065FXXS4AYE2_A065FOLE4AYE2_20240401.tar.md5")

	_, err := BuildFlashPlan(files, "whatever", approvedSession(t))
	if err == nil {
		t.Fatal("expected an unknown install mode to be rejected")
	}
}

func TestEngineStatusReportsMissingBinary(t *testing.T) {
	withTempState(t)
	t.Setenv("AUTOROOT_SAMLOADER", filepath.Join(t.TempDir(), "absent.exe"))

	// An empty pin map means nothing is verified, so status must say so
	// rather than reporting a usable engine.
	prev, had := samloaderPins[samloaderVersion]
	delete(samloaderPins, samloaderVersion)
	t.Cleanup(func() {
		if had {
			samloaderPins[samloaderVersion] = prev
		}
	})

	status := engineStatus()
	if status["available"] != false {
		t.Errorf("expected an unavailable engine, got %v", status)
	}
	if status["reason"] == "" {
		t.Error("an unavailable engine must explain why")
	}
}
