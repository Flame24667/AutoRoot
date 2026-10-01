package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Keep large artifacts in an operator-selected volume or project firmware/;
// no developer-specific absolute paths are embedded in the public source.
const (
	defaultFirmwareRoot = "firmware"
	defaultToolsRoot    = "tools"
	stateDirName        = "state"
	logDirName          = "logs"
	cacheDirName        = "cache"
)

// envFirmwareRoot lets an operator override the firmware root without editing
// code. Download preparation separately checks volume and space requirements.
func envFirmwareRoot() string { return strings.TrimSpace(os.Getenv("AUTOROOT_FIRMWARE_ROOT")) }

func isSystemDriveLetter(letter byte) bool {
	vol := strings.ToUpper(os.Getenv("SystemDrive"))
	if vol == "" {
		vol = "C:"
	}
	return strings.ToUpper(string(letter)) == vol[:1]
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
// Order of precedence: explicit override, the detected source project's
// firmware folder, then a folder beside the executable.
func resolveFirmwareRoot() string {
	if override := envFirmwareRoot(); override != "" {
		return filepath.Clean(override)
	}

	cwd, _ := os.Getwd()
	for _, root := range []string{cwd, filepath.Dir(appDir()), appDir()} {
		if _, err := os.Stat(filepath.Join(root, "backend", "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(root, "frontend", "package.json")); err == nil {
				return filepath.Join(root, defaultFirmwareRoot)
			}
		}
	}

	// Packaged fallback: beside the executable; use an explicit volume override.
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

// stateDirOverride redirects the state directory, used by tests so a run
// never touches the real session file. Empty in production.
var stateDirOverride string

// writableStateDir holds the state machine, logs and caches. It is intentionally
// small, so it may live on the system drive even when firmware cannot.
func writableStateDir() string {
	if stateDirOverride != "" {
		return stateDirOverride
	}
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

// toolsDir locates the flashing engine. The binary lives in bin/ during
// development and in resources/bin/ once packaged, while the engine itself is
// kept in the project's tools folder, so the search walks upward from the
// executable.
//
// Only pre-existing folders are accepted, otherwise this function would create
// an empty bin\tools on the first call and stop the walk before it ever reached
// the real one.
func toolsDir() string {
	if override := strings.TrimSpace(os.Getenv("AUTOROOT_TOOLS_DIR")); override != "" {
		if ensureDir(override) {
			return override
		}
	}

	if dir := appDir(); dir != "" {
		cur := dir
		// Walk up a few levels so both bin\ and resources\bin\ resolve to the
		// project's tools folder.
		for i := 0; i < 4 && cur != ""; i++ {
			candidate := filepath.Join(cur, defaultToolsRoot)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}

	// Nothing existing: create the folder beside the executable so the operator
	// has an obvious place to drop the engine.
	fallback := filepath.Join(appDirOrDot(), defaultToolsRoot)
	ensureDir(fallback)
	return fallback
}

func appDirOrDot() string {
	if dir := appDir(); dir != "" {
		return dir
	}
	return "."
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
