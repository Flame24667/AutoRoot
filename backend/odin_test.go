package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsWindowsExecutableRejectsSmallPointerFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Odin3.exe")
	if err := os.WriteFile(path, []byte("version https://git-lfs.github.com/spec/v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if isWindowsExecutable(path) {
		t.Fatal("expected a Git LFS pointer to be rejected")
	}
}

func TestIsWindowsExecutableAcceptsLargePEHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool.exe")
	data := make([]byte, 1024*1024+1)
	data[0], data[1] = 'M', 'Z'
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if !isWindowsExecutable(path) {
		t.Fatal("expected a sufficiently large MZ executable to be accepted")
	}
}

func TestFlashWithOdinRejectsHomeCSCForInitialRoot(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}
	for _, slot := range []string{"AP", "BL", "CP", "CSC"} {
		name := slot + "_firmware.tar.md5"
		if slot == "CSC" {
			name = "HOME_CSC_firmware.tar.md5"
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		files[slot] = path
	}

	if _, errText := flashWithOdin("serial", files, "initial-root"); errText == "" {
		t.Fatal("expected HOME_CSC to be rejected for initial root")
	}
}

func TestFlashWithOdinRejectsStandardCSCForRootedUpdate(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}
	for _, slot := range []string{"AP", "BL", "CP", "CSC"} {
		path := filepath.Join(dir, slot+"_firmware.tar.md5")
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		files[slot] = path
	}

	if _, errText := flashWithOdin("serial", files, "rooted-update"); errText == "" {
		t.Fatal("expected standard CSC to be rejected for rooted update")
	}
}
