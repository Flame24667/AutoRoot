package main

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPkgAP = "AP_A065FXXS4AYE2_A065FOLE4AYE2_26021ABCDEFGOP_20240401_000000.8102.tar.md5"
const testPkgBL = "BL_A065FXXS4AYE2_A065FXXS4AYE1_26021ABCDEFGOP_20240401_000000.8102.tar.md5"
const testPkgCP = "CP_A065FXXS4AYE2_A065FXXS4AYE1_26021ABCDEFGOP_20240401_000000.8102.tar.md5"
const testPkgCSC = "HOME_CSC_A065FXXS4AYE2_A065FOLE4AYE2_26021ABCDEFGOP_20240401_000000.8102.tar.md5"

// buildFirmwareZip creates a Samsung-shaped package whose .tar.md5 members carry
// correct internal digests, so validation exercises the real happy path.
func buildFirmwareZip(t *testing.T, dir string, members map[string]string) string {
	t.Helper()

	stage := t.TempDir()
	zipPath := filepath.Join(dir, "AP_A065FXXS4AYE2_A065FOLE4AYE2_26021ABCDEFGOP_20240401_000000.8102.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, payload := range members {
		var data []byte
		if strings.HasSuffix(name, ".tar.md5") {
			sum := md5.Sum([]byte(payload))
			// Pad the .tar to a plausible multi-hundred-MB scale is unnecessary;
			// the .md5 content itself must simply be large enough to pass the
			// size floor, so pad the payload instead.
			big := payload + strings.Repeat("0", 2048)
			sum = md5.Sum([]byte(big))
			data = []byte(hex.EncodeToString(sum[:]) + "  " + name)
			if err := writeZipEntry(zw, strings.TrimSuffix(name, ".md5"), []byte(big)); err != nil {
				t.Fatal(err)
			}
		} else {
			data = []byte(payload)
		}
		if err := writeZipEntry(zw, name, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = stage
	return zipPath
}

func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func validMembers() map[string]string {
	return map[string]string{
		testPkgAP:  strings.Repeat("A", 3000),
		testPkgBL:  strings.Repeat("B", 3000),
		testPkgCP:  strings.Repeat("C", 3000),
		testPkgCSC: strings.Repeat("D", 3000),
	}
}

func targetProfile() FirmwareProfile {
	return FirmwareProfile{Model: "SM-A065F", CSC: "XID", DeviceBinary: "4"}
}

// useTestSizeFloor lowers the "this file is too small to be real firmware"
// threshold so small in-memory fixtures can exercise the surrounding logic.
// The production value stays 1 MB.
func useTestSizeFloor(t *testing.T) {
	t.Helper()
	prev := minArchiveBytes
	minArchiveBytes = 16
	t.Cleanup(func() { minArchiveBytes = prev })
}

func TestValidateAcceptsMatchingPackage(t *testing.T) {
	useTestSizeFloor(t)
	dir := t.TempDir()
	zipPath := buildFirmwareZip(t, dir, validMembers())

	v := ValidateFirmwareArchive(zipPath, targetProfile(), filepath.Join(dir, "out"))
	if !v.OK {
		t.Fatalf("expected valid package, got errors: %v", v.Errors)
	}
	if v.ModelCode != "A065F" {
		t.Fatalf("expected model code A065F, got %q", v.ModelCode)
	}
	if v.PkgBinary != "4" {
		t.Fatalf("expected binary 4, got %q", v.PkgBinary)
	}
	for _, slot := range RequiredSlots {
		if _, ok := v.Sizes[slot]; !ok {
			t.Fatalf("expected slot %s to be present", slot)
		}
	}
}

func TestValidateRejectsDifferentModel(t *testing.T) {
	useTestSizeFloor(t)
	dir := t.TempDir()
	members := validMembers()
	delete(members, testPkgAP)
	members["AP_A055FXXS4AYE2_A055FOLE4AYE2_26021ABCDEFGOP_20240401_000000.8102.tar.md5"] = strings.Repeat("A", 3000)

	zipPath := filepath.Join(dir, "AP_A065FXXS4AYE2_A065FOLE4AYE2_26021ABCDEFGOP_20240401_000000.8102.zip")
	rewriteZip(t, zipPath, members)

	v := ValidateFirmwareArchive(zipPath, targetProfile(), filepath.Join(dir, "out"))
	if v.OK {
		t.Fatal("expected package for another model to be rejected")
	}
	if !containsSubstring(v.Errors, "is built for A055F") {
		t.Fatalf("expected a model mismatch error, got %v", v.Errors)
	}
}

// A055F is a real Galaxy A05s; its firmware must never be flashed to an A065F.
func rewriteZip(t *testing.T, path string, members map[string]string) {
	t.Helper()
	os.Remove(path)
	_ = buildFirmwareZip(t, filepath.Dir(path), members)
	os.Rename(filepath.Join(filepath.Dir(path), filepath.Base(path)), path)
}

func TestValidateRejectsAntiRollback(t *testing.T) {
	useTestSizeFloor(t)
	dir := t.TempDir()
	// Binary 3 is below the device's binary 4.
	zipPath := filepath.Join(dir, "AP_A065FXXS3AYE2_A065FOLE4AYE2_26021ABCDEFGOP_20240401_000000.8102.zip")
	_ = buildFirmwareZipNamed(t, dir, filepath.Base(zipPath), validMembers())

	v := ValidateFirmwareArchive(zipPath, targetProfile(), filepath.Join(dir, "out"))
	if v.OK {
		t.Fatal("expected binary 3 package to be rejected for a binary 4 device")
	}
	if !containsSubstring(v.Errors, "anti-rollback") {
		t.Fatalf("expected an anti-rollback error, got %v", v.Errors)
	}
}

func TestValidateAcceptsHigherBinary(t *testing.T) {
	useTestSizeFloor(t)
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "AP_A065FXXS5AYE2_A065FOLE4AYE2_26021ABCDEFGOP_20240401_000000.8102.zip")
	_ = buildFirmwareZipNamed(t, dir, filepath.Base(zipPath), validMembers())

	v := ValidateFirmwareArchive(zipPath, targetProfile(), filepath.Join(dir, "out"))
	if !v.OK {
		t.Fatalf("expected binary 5 package to be accepted, got %v", v.Errors)
	}
}

func buildFirmwareZipNamed(t *testing.T, dir, name string, members map[string]string) string {
	t.Helper()
	zipPath := filepath.Join(dir, name)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for member, payload := range members {
		var data []byte
		if strings.HasSuffix(member, ".tar.md5") {
			big := payload + strings.Repeat("0", 2048)
			sum := md5.Sum([]byte(big))
			data = []byte(hex.EncodeToString(sum[:]) + "  " + member)
			if err := writeZipEntry(zw, strings.TrimSuffix(member, ".md5"), []byte(big)); err != nil {
				t.Fatal(err)
			}
		} else {
			data = []byte(payload)
		}
		if err := writeZipEntry(zw, member, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return zipPath
}

func TestValidateRejectsCorruptedMD5(t *testing.T) {
	useTestSizeFloor(t)
	dir := t.TempDir()
	zipPath := buildFirmwareZip(t, dir, validMembers())
	profile := targetProfile()

	// Extract once, then tamper with the extracted AP payload so the digest
	// declared by its .tar.md5 no longer matches - exactly what a corrupted or
	// truncated transfer looks like. Re-validating into the same folder proves
	// the check reads the payload that is actually on disk.
	v := ValidateFirmwareArchive(zipPath, profile, filepath.Join(dir, "out"))
	if !v.OK {
		t.Fatalf("baseline package should be valid, got %v", v.Errors)
	}
	apTar := strings.TrimSuffix(v.Sizes[SlotAP].Path, ".md5")
	apInfo, err := os.Stat(apTar)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt in place, preserving the byte count: this is the realistic
	// failure mode (a flipped sector, a resumed download stitched together
	// wrongly) and the one the size check cannot mask.
	if err := os.WriteFile(apTar, bytes.Repeat([]byte("Z"), int(apInfo.Size())), 0o644); err != nil {
		t.Fatal(err)
	}

	again := ValidateFirmwareArchive(zipPath, profile, filepath.Join(dir, "out"))
	if again.OK {
		t.Fatal("expected a tampered AP payload to fail md5 verification")
	}
	if !containsSubstring(again.Errors, "md5 mismatch") {
		t.Fatalf("expected an md5 mismatch, got %v", again.Errors)
	}
}

func TestValidateRejectsHTMLCapture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AP_A065FXXS4AYE2_A065FOLE4AYE2_x.zip")
	body := "<!DOCTYPE html><html><body>Google Drive - Quota exceeded</body></html>"
	body += strings.Repeat(" ", 2000)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	v := ValidateFirmwareArchive(path, targetProfile(), filepath.Join(dir, "out"))
	if v.OK {
		t.Fatal("expected an HTML capture to be rejected")
	}
	if !containsSubstring(v.Errors, "HTML") {
		t.Fatalf("expected an HTML error, got %v", v.Errors)
	}
}

func TestValidateRejectsMissingSlot(t *testing.T) {
	useTestSizeFloor(t)
	dir := t.TempDir()
	members := validMembers()
	delete(members, testPkgCP)
	zipPath := buildFirmwareZipNamed(t, dir, "AP_A065FXXS4AYE2_A065FOLE4AYE2_x.zip", members)

	v := ValidateFirmwareArchive(zipPath, targetProfile(), filepath.Join(dir, "out"))
	if v.OK {
		t.Fatal("expected a package missing CP to be rejected")
	}
	if !containsSubstring(v.Errors, "missing the CP") {
		t.Fatalf("expected a missing-CP error, got %v", v.Errors)
	}
}

func TestValidateWarnsOnHomeCSC(t *testing.T) {
	useTestSizeFloor(t)
	dir := t.TempDir()
	zipPath := buildFirmwareZip(t, dir, validMembers())
	v := ValidateFirmwareArchive(zipPath, targetProfile(), filepath.Join(dir, "out"))
	if !v.OK {
		t.Fatalf("expected valid package, got %v", v.Errors)
	}
	if !containsSubstring(v.Warnings, "HOME_CSC") {
		t.Fatalf("expected a HOME_CSC warning, got %v", v.Warnings)
	}
}

func TestBinaryFromName(t *testing.T) {
	cases := map[string]string{
		"AP_A065FXXS4AYE2_A065FOLE4AYE2_26021ABC_20240401.zip": "4",
		"AP_A065FXXS5AYE2_A065FOLE4AYE2_26021ABC_20240401.zip": "5",
		"BL_A065FXXSAYH2_A065FXXSAYH1_26021ABC_20240401.zip":   "A",
		"nonsense.zip":                                          "",
	}
	for name, want := range cases {
		if got := binaryFromName(name); got != want {
			t.Errorf("binaryFromName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestModelFromName(t *testing.T) {
	cases := map[string]string{
		"AP_A065FXXS4AYE2_x.zip": "A065F",
		"AP_A055FXXS4AYE2_x.zip": "A055F",
		"garbage.zip":            "",
	}
	for name, want := range cases {
		if got := samsungModelFromName(name); got != want {
			t.Errorf("samsungModelFromName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestCompareRevisions(t *testing.T) {
	if compareRevisions("3", "4") != -1 {
		t.Error("3 must sort below 4")
	}
	if compareRevisions("4", "4") != 0 {
		t.Error("4 must equal 4")
	}
	if compareRevisions("5", "4") != 1 {
		t.Error("5 must sort above 4")
	}
	if compareRevisions("A", "9") != 1 {
		t.Error("A must sort above 9 in base 36")
	}
}

func TestDeviceFirmwareDirTargetsD(t *testing.T) {
	dir := deviceFirmwareDir("SM-A065F")
	if !strings.Contains(dir, "SM-A065F") {
		t.Fatalf("device firmware dir must be model-scoped, got %q", dir)
	}
	if strings.Contains(dir, "..") {
		t.Fatalf("device firmware dir must be sanitized, got %q", dir)
	}
}

func TestSanitizeModelDirRejectsPathTraversal(t *testing.T) {
	for _, bad := range []string{"../../etc", "SM-A065F/../x", "SM A065F", "SM-A065F\\x"} {
		if got := sanitizeModelDir(bad); got != "" && strings.Contains(got, string(filepath.Separator)) {
			t.Fatalf("sanitizeModelDir(%q) leaked a separator: %q", bad, got)
		}
	}
	if got := sanitizeModelDir("SM-A065F"); got != "SM-A065F" {
		t.Fatalf("sanitizeModelDir(SM-A065F) = %q", got)
	}
}

func containsSubstring(hay []string, needle string) bool {
	for _, h := range hay {
		if strings.Contains(h, needle) {
			return true
		}
	}
	return false
}
