package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Workspace layout. Large artifacts (firmware, dumps, logs) must never land on
// the system drive: the C: volume on the target machine is nearly full and
// Samsung packages are ~5.8 GB, so a C: destination fails mid-download.
const (
	defaultFirmwareRoot = `D:\Data Kelola IT\Firmware`
	defaultToolsRoot    = "tools"
	stateDirName        = "state"
	logDirName          = "logs"
	cacheDirName        = "cache"
)

// envFirmwareRoot lets an operator override the firmware root without editing
// code. It is validated to be an absolute, non-system-drive path.
func envFirmwareRoot() string { return strings.TrimSpace(os.Getenv("AUTOROOT_FIRMWARE_ROOT")) }

func isSystemDriveLetter(letter byte) bool {
	return letter == 'C' || letter == 'D'
}

// sameVolumeRoot reports whether path sits on the Windows system drive, which
// is the volume we must keep free of multi-gigabyte payloads.
func onSystemDrive(path string) bool {
	vol := filepath.VolumeName(path)
	if vol == "" {
		return true
	}
	letter := vol[0]
	return isSystemDriveLetter(letter)
}

// resolveFirmwareRoot returns the directory that holds every firmware download.
// Order of precedence: explicit override, then the packaged default, then a
// development-tree fallback so tests and local runs still work.
func resolveFirmwareRoot() string {
	if override := envFirmwareRoot(); override != "" {
		return filepath.Clean(override)
	}

	if runtime.GOOS == "windows" {
		// The machine is provisioned with a real D: volume; prefer it whenever
		// it is present and has enough headroom for a full package.
		if volFreeOK(defaultFirmwareRoot) {
			return defaultFirmwareRoot
		}
	}

	// Non-Windows or missing D: -> keep everything beside the executable.
	exeDir := appDir()
	if exeDir != "" {
		return filepath.Join(exeDir, "firmware")
	}
	return "firmware"
}

// deviceFirmwareDir is the per-model download target, e.g. D:\...\Firmware\SM-A065F.
func deviceFirmwareDir(model string) string {
	safe := sanitizeModelDir(model)
	if safe == "" {
		return resolveFirmwareRoot()
	}
	return filepath.Join(resolveFirmwareRoot(), safe)
}

func sanitizeModelDir(model string) string {
	model = strings.ToUpper(strings.TrimSpace(model))
	if model == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range model {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			return ""
		}
	}
	return b.String()
}

// appDir is the directory holding the running binary; empty when it cannot be
// determined, in which case callers fall back to the working directory.
func appDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exePath)
}

// writableStateDir holds the state machine, logs and caches. It is intentionally
// small, so it may live on the system drive even when firmware cannot.
func writableStateDir() string {
	if dir := appDir(); dir != "" {
		candidate := filepath.Join(dir, stateDirName)
		if ensureDir(candidate) {
			return candidate
		}
	}
	if ensureDir(stateDirName) {
		return stateDirName
	}
	return "."
}

func logDir() string {
	dir := filepath.Join(writableStateDir(), logDirName)
	ensureDir(dir)
	return dir
}

func cacheDir() string {
	dir := filepath.Join(writableStateDir(), cacheDirName)
	ensureDir(dir)
	return dir
}

func toolsDir() string {
	if dir := appDir(); dir != "" {
		candidate := filepath.Join(dir, defaultToolsRoot)
		if ensureDir(candidate) {
			return candidate
		}
	}
	dir := filepath.Join(writableStateDir(), defaultToolsRoot)
	ensureDir(dir)
	return dir
}

func ensureDir(path string) bool {
	if path == "" {
		return false
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// volFreeOK reports whether the volume holding path has at least minFree bytes
// available. Used to keep multi-gigabyte downloads off an exhausted C:.
func volFreeOK(path string) bool {
	// Probe the nearest existing ancestor so a not-yet-created folder still
	// resolves to the right volume.
	probe := path
	for {
		if info, err := os.Stat(probe); err == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return false
		}
		probe = parent
	}
	if _, err := os.Stat(probe); err != nil {
		return false
	}
	return volumeFreeBytes(probe) >= minFirmwareFreeBytes
}

// minFirmwareFreeBytes is the headroom required before starting a download:
// one full package (5.81 GB) plus 20% for extraction of the unpacked parts.
const minFirmwareFreeBytes = int64(7_000_000_000)
