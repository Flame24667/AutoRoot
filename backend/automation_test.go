package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"crypto/md5"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func samsungTarFixture(t *testing.T, name string) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	data := bytes.Repeat([]byte("X"), 8192)
	if err := tw.WriteHeader(&tar.Header{Name: "test.img", Mode: 0600, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum(b.Bytes())
	return append(b.Bytes(), []byte(fmt.Sprintf("%x  %s\n", sum, name))...)
}

func TestSamsungEmbeddedMD5AndTamper(t *testing.T) {
	path := filepath.Join(t.TempDir(), "BL_A065FXXS4AYE2.tar.md5")
	data := samsungTarFixture(t, filepath.Base(path))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyTarMD5(path); err != nil {
		t.Fatal(err)
	}
	data[600] ^= 1
	os.WriteFile(path, data, 0600)
	if err := verifyTarMD5(path); err == nil || !strings.Contains(err.Error(), "md5 mismatch") {
		t.Fatalf("tamper accepted: %v", err)
	}
}

func TestEmbeddedMD5RejectsDifferentZipMember(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AP_A065FXXS4AYE2.tar.md5")
	data := samsungTarFixture(t, filepath.Base(path))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	crc := crc32.ChecksumIEEE(data)
	if err := verifyEmbeddedMD5(path, crc); err != nil {
		t.Fatal(err)
	}
	if err := verifyEmbeddedMD5(path, crc^1); err == nil {
		t.Fatal("different ZIP member accepted")
	}
}

func TestSamsungCSCBuildInformationIsIncludedInMD5(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CSC_OLE_A065FOLE4AYE2.tar.md5")
	data := samsungTarFixture(t, filepath.Base(path))
	_, n, err := func() (string, int64, error) { os.WriteFile(path, data, 0600); return embeddedMD5(path) }()
	if err != nil {
		t.Fatal(err)
	}
	prefix := append(data[:n:n], []byte(fmt.Sprintf("Show the build information\nRBS BUILD_ID:96133963\noriginal_tar_file_size:%d\n", n))...)
	sum := md5.Sum(prefix)
	complete := append(prefix, []byte(fmt.Sprintf("%x  %s\n", sum, filepath.Base(path)))...)
	if err := os.WriteFile(path, complete, 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyEmbeddedMD5(path, crc32.ChecksumIEEE(complete)); err != nil {
		t.Fatal(err)
	}
	complete[n+5] ^= 1
	os.WriteFile(path, complete, 0600)
	if err := verifyEmbeddedMD5(path); err == nil {
		t.Fatal("modified metadata accepted")
	}
}

func TestVendorZipNameAndRealCSCWithBothSlots(t *testing.T) {
	useTestSizeFloor(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "SM-A065F_1_20250513151550_u4gbht1wk2_fac.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	names := []string{"AP_A065FXXS4AYE2.tar.md5", "BL_A065FXXS4AYE2.tar.md5", "CP_A065FXXS4AYE1.tar.md5", "CSC_OLE_A065FOLE4AYE2_REV00.tar.md5", "HOME_CSC_OLE_A065FOLE4AYE2_REV00.tar.md5"}
	for _, name := range names {
		if err := writeZipEntry(zw, name, samsungTarFixture(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	zw.Close()
	f.Close()
	profile := FirmwareProfile{Model: "SM-A065F", CSC: "XID", CSCBuild: "A065FOLE4AYE2", DeviceBinary: "4"}
	v := ValidateFirmwareArchive(path, profile, filepath.Join(dir, "out"))
	if !v.OK {
		t.Fatal(v.Errors)
	}
	if !strings.HasPrefix(v.Sizes[SlotCSC].FileName, "CSC_") {
		t.Fatal("HOME_CSC replaced wiping CSC")
	}
	if v.CSCInPackage != "XID" {
		t.Fatal("installed OMC proof not recorded")
	}
	if v.PkgBinary != "4" || v.ModelCode != "A065F" {
		t.Fatal("members were not authoritative")
	}
}

func TestDatabaseRequiresExactVariantRegionBuildAndOMC(t *testing.T) {
	data := []byte(`{"devices":[{"model":"SM-A065F","firmware":[{"region":"XID","pda":"A065FXXS4AYE2","csc":"A065FOLE4AYE2","bootloaderBinary":"4"}]}]}`)
	s := NewSession("serial", "SM-A065F", "XID", "4")
	s.Provenance["deviceBuild"] = "A065FXXS4AYE2"
	s.Provenance["deviceCSCBuild"] = "A065FOLE4AYE2"
	if _, err := selectCatalog(data, s); err != nil {
		t.Fatal(err)
	}
	s.Model = "SM-A065FD"
	if _, err := selectCatalog(data, s); err == nil {
		t.Fatal("variant prefix matched")
	}
	s.Model = "SM-A065F"
	s.CSC = "BTU"
	if _, err := selectCatalog(data, s); err == nil {
		t.Fatal("wrong region matched")
	}
	s.CSC = "XID"
	s.Provenance["deviceCSCBuild"] = "unknown"
	if _, err := selectCatalog(data, s); err == nil {
		t.Fatal("unknown OMC matched")
	}
}

func TestDatabaseRejectsAmbiguousRecords(t *testing.T) {
	data := []byte(`{"devices":[{"model":"SM-A065F","firmware":[{"region":"XID","pda":"A065FXXS4AYE2","csc":"A065FOLE4AYE2","bootloaderBinary":"4"},{"region":"XID","pda":"A065FXXS4AYE2","csc":"A065FOLE4AYE2","bootloaderBinary":"4"}]}]}`)
	s := NewSession("serial", "SM-A065F", "XID", "4")
	s.Provenance["deviceBuild"] = "A065FXXS4AYE2"
	s.Provenance["deviceCSCBuild"] = "A065FOLE4AYE2"
	if _, err := selectCatalog(data, s); err == nil {
		t.Fatal("ambiguous record accepted")
	}
}

func TestFlashApprovalBoundToActualFiles(t *testing.T) {
	withTempState(t)
	s := NewSession("serial", "SM-A065F", "XID", "4")
	s.Stage = StageApprovedFlash
	files := writeFakeSlotFiles(t, t.TempDir(), "CSC_A065FXXS4AYE2.tar.md5")
	for slot, path := range files {
		s.Artifacts["slot-"+string(slot)] = path
	}
	s.Artifacts["patchedAP"] = files[SlotAP]
	hash, err := flashFingerprint(s)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.GrantApproval("test", hash); err != nil {
		t.Fatal(err)
	}
	if err = requireBoundApproval(s); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(files[SlotCP], []byte("changed"), 0600)
	if err = requireBoundApproval(s); err == nil {
		t.Fatal("file change accepted")
	}
	s.Stage = StageFlashing
	if err = requireBoundApproval(s); err == nil {
		t.Fatal("interrupted flash accepted")
	}
}

func TestDestructiveActionsRejectMissingFinalConsent(t *testing.T) {
	if _, err := executeFlash(map[string]interface{}{}); err == "" {
		t.Fatal("flash allowed without final confirmation")
	}
	if _, err := probeEngine(map[string]interface{}{}); err == "" {
		t.Fatal("reboot allowed without confirmation")
	}
}

func TestRandomTarIsNotMagiskAP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patched.tar")
	os.WriteFile(path, []byte("not tar"), 0600)
	if err := inspectPatchedAP(path); err == nil {
		t.Fatal("random patch accepted")
	}
}

func TestDownloadIdentityRejectsEmptyOrForeignDevice(t *testing.T) {
	withTempState(t)
	s := NewSession("serial", "SM-A065F", "XID", "4")
	for _, info := range []map[string]interface{}{{"modelName": "", "serialNumber": ""}, {"modelName": "SM-A055F", "serialNumber": "serial"}, {"modelName": "SM-A065F", "serialNumber": "other"}} {
		if err := validateDownloadIdentity(s, info); err == nil {
			t.Fatal("empty or foreign identity accepted")
		}
	}
	if err := validateDownloadIdentity(s, map[string]interface{}{"modelName": "SM-A065F", "serialNumber": "serial"}); err != nil {
		t.Fatal(err)
	}
}

// Explicit opt-in local integration test: never uses ADB, installs or flashes.
func TestLocalA065FFirmwareOptional(t *testing.T) {
	archive := os.Getenv("AUTOROOT_TEST_A065F_ARCHIVE")
	extractDir := os.Getenv("AUTOROOT_TEST_A065F_EXTRACTED")
	if archive == "" || extractDir == "" {
		t.Skip("local firmware integration test not requested")
	}
	v := ValidateFirmwareArchive(archive, FirmwareProfile{Model: "SM-A065F", CSC: "XID", CSCBuild: "A065FOLE4AYE2", DeviceBinary: "4"}, extractDir)
	if !v.OK {
		t.Fatal(v.Errors)
	}
	if len(v.Sizes) != 4 || v.CSCInPackage != "XID" || v.PkgBinary != "4" {
		t.Fatalf("unexpected validation: %+v", v)
	}
	t.Logf("Validated actual local AP/BL/CP/CSC: model=%s binary=%s activeCSC=%s", v.ModelCode, v.PkgBinary, v.CSCInPackage)
}
