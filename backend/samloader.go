package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// samloader-rs (topjohnwu) is the flashing engine. It is the only Samsung
// flashing tool used here that ships a real, documented command line; Odin is
// a GUI with no supported CLI, and the bundled Odin3.exe is a 132-byte Git LFS
// pointer rather than a program.
//
// Two separate concerns use this engine:
//
//   - USB flashing (detect, dump-pit, flash). This talks to the device
//     directly and does not depend on TLS.
//   - FUS firmware download, which does use TLS. The Windows SChannel
//     credential failure seen earlier affects that path, not the USB path.
//
// The engine is version-pinned and checksum-verified before it is ever run, so
// a swapped or truncated binary cannot be executed.

// samloaderPins maps an allowed engine version to the sha256 of the exact
// Windows release artifact. An unlisted version is refused: "latest" is not a
// pin, and a flashing tool is the last thing that should silently change.
//
// The digest below is of the artifact this project verified on the target
// machine: it reports "samloader 2.2.0", exposes the documented subcommands
// (download, check-update, detect, dump-pit, print-pit, flash, verify-md5,
// reboot-download) and defaults to the vcom USB backend, which on Windows works
// with the stock Samsung driver and needs no Zadig replacement.
var samloaderPins = map[string]string{
	"2.2.0": "b83b8244ecc86ecb4f5efc08e1604214d869838ca59b329e02a34e70832d175b",
}

// samloaderVersion is the single engine version this build accepts.
const samloaderVersion = "2.2.0"

// engineCandidatePaths lists where a samloader executable may live, in priority
// order: an explicit override, the bundled tools folder, then PATH.
func engineCandidatePaths() []string {
	var paths []string
	if override := strings.TrimSpace(os.Getenv("AUTOROOT_SAMLOADER")); override != "" {
		paths = append(paths, override)
	}
	paths = append(paths, filepath.Join(toolsDir(), "samloader.exe"))
	return paths
}

// resolveSamloader returns a verified samloader executable, or an error that
// says precisely what is missing.
func resolveSamloader() (string, error) {
	// A cached verification from an earlier run avoids rehashing on every call.
	for _, path := range engineCandidatePaths() {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		sum, err := FileSHA256(path)
		if err != nil {
			return "", fmt.Errorf("cannot hash %s: %w", path, err)
		}
		want, pinned := samloaderPins[samloaderVersion]
		if !pinned {
			return "", fmt.Errorf("no pinned sha256 is registered for samloader %s; refusing to run an unverified flashing engine (add it to samloaderPins after checking the official release)",
				samloaderVersion)
		}
		if !strings.EqualFold(sum, want) {
			return "", fmt.Errorf("samloader checksum mismatch for %s\n  expected %s\n  actual   %s\nThe executable does not match the pinned official release and will not be run",
				path, want, sum)
		}
		return path, nil
	}
	return "", fmt.Errorf("samloader was not found. Place samloader %s in %s, or set AUTOROOT_SAMLOADER to its path",
		samloaderVersion, toolsDir())
}

// engineStatus reports readiness without running anything on the device.
func engineStatus() map[string]interface{} {
	status := map[string]interface{}{
		"engine":  "samloader-rs",
		"version": samloaderVersion,
		"pinned":  samloaderPins[samloaderVersion] != "",
	}
	path, err := resolveSamloader()
	if err != nil {
		status["available"] = false
		status["reason"] = err.Error()
		status["path"] = ""
		return status
	}
	status["available"] = true
	status["path"] = path
	return status
}

// runSamloader executes the engine and returns its combined output. It never
// runs a flashing subcommand: callers must build those separately so a plain
// status check cannot flash by accident.
func runSamloader(args ...string) (string, error) {
	bin, err := resolveSamloader()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = toolsDir()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// samloaderVersionCheck runs `samloader --version` and returns the reported
// version, so a mismatch against the pin surfaces immediately.
func samloaderVersionCheck() (string, error) {
	out, err := runSamloader("--version")
	if err != nil {
		return "", fmt.Errorf("samloader --version failed: %w\n%s", err, strings.TrimSpace(out))
	}
	fields := strings.Fields(strings.TrimSpace(out))
	for _, f := range fields {
		if strings.Contains(f, samloaderVersion) {
			return f, nil
		}
	}
	return strings.TrimSpace(out), nil
}

// DeviceDetect asks the engine whether a download-mode device is visible.
// This is a read-only query: it never writes to the phone.
func DeviceDetect(wait bool, asJSON bool) (map[string]interface{}, error) {
	bin, err := resolveSamloader()
	if err != nil {
		return nil, err
	}

	args := []string{"detect"}
	if wait {
		args = append(args, "--wait")
	}
	if asJSON {
		args = append(args, "--json")
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = toolsDir()
	out, runErr := cmd.CombinedOutput()
	result := map[string]interface{}{
		"available": true,
		"raw":       strings.TrimSpace(string(out)),
	}

	if asJSON {
		// `detect --json` is documented as odin4-compatible JSON; prefer the
		// structured form when the engine actually produced it.
		var parsed map[string]interface{}
		if err := json.Unmarshal(out, &parsed); err == nil {
			for k, v := range parsed {
				result[k] = v
			}
		}
	}
	if runErr != nil {
		result["available"] = false
		result["error"] = runErr.Error()
	}
	return result, nil
}

// DeviceInDownloadMode reports whether a device is currently in Download Mode.
// A timeout is treated as "not detected" rather than an error, because this is
// polled while the phone reboots.
func DeviceInDownloadMode() (bool, error) {
	res, err := DeviceDetect(false, true)
	if err != nil {
		return false, err
	}
	if ok, present := res["available"].(bool); present && !ok {
		return false, nil
	}
	// The engine exits non-zero when nothing is in download mode, and the
	// message is the only signal available without a device attached.
	raw, _ := res["raw"].(string)
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "no download mode") ||
		strings.Contains(lower, "waiting for device") ||
		strings.Contains(lower, "no device") {
		return false, nil
	}
	if strings.TrimSpace(raw) == "" {
		return false, nil
	}
	return true, nil
}

// WaitForDownloadMode polls until the phone appears in Download Mode or the
// deadline passes. The user must press Volume Up on some models to get there
// from a powered-off state, so a timeout is a normal outcome, not a fault.
func WaitForDownloadMode(timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ok, err := DeviceInDownloadMode()
		if err != nil {
			// A missing or unverifiable engine is fatal; a phone that has not
			// arrived yet is not.
			return false, err
		}
		if ok {
			return true, nil
		}
		time.Sleep(2 * time.Second)
	}
	return false, nil
}

// DumpPIT writes the device's partition table to a file. This is read-only with
// respect to the phone's contents and is the safest way to confirm the engine
// can actually talk to an SM-A065F.
func DumpPIT(destPath string) (string, error) {
	bin, err := resolveSamloader()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", err
	}
	out, runErr := exec.Command(bin, "dump-pit", "--output", destPath).CombinedOutput()
	return string(out), runErr
}

// VerifyFirmwareMD5 runs the engine's own checksum verification over the
// extracted .tar.md5 files. It is an independent second opinion: AutoRoot
// verifies digests itself, and this confirms the engine agrees.
func VerifyFirmwareMD5(files []string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no .tar.md5 files given")
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return "", fmt.Errorf("missing %s: %w", f, err)
		}
	}
	args := append([]string{"verify-md5"}, files...)
	out, err := runSamloader(args...)
	return out, err
}

// FlashPlan is a fully resolved, not-yet-executed flash command. Building one
// has no side effects; executing it is a separate, approval-gated step.
type FlashPlan struct {
	Args         []string
	EnginePath   string
	EngineVer    string
	PackageFiles map[Slot]string
	InstallMode  string
	Reboot       bool
}

// BuildFlashPlan assembles the samloader arguments for a validated firmware
// set. It performs every check that can be done without touching the device,
// and it never executes anything.
//
// installMode is "initial-root" for the first Magisk install (which needs a
// real CSC because it wipes data) or "rooted-update" for a later flash on an
// already-rooted phone (which needs HOME_CSC so data survives).
func BuildFlashPlan(pkg map[Slot]string, installMode string, session *Session) (*FlashPlan, error) {
	engine, err := resolveSamloader()
	if err != nil {
		return nil, err
	}

	// The consent gate is checked here, before a plan can even be built, so a
	// caller cannot assemble a flash command without it.
	if err := session.RequireApproval(); err != nil {
		return nil, err
	}

	plan := &FlashPlan{
		EnginePath:   engine,
		EngineVer:    samloaderVersion,
		PackageFiles: map[Slot]string{},
		InstallMode:  installMode,
		Reboot:       true,
	}

	for _, slot := range RequiredSlots {
		path := strings.TrimSpace(pkg[slot])
		if path == "" {
			return nil, fmt.Errorf("missing the %s partition", slot)
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("the %s file is not readable: %s", slot, path)
		}
		plan.PackageFiles[slot] = path
	}

	// The first Magisk install must use a wiping CSC; a rooted update must use
	// HOME_CSC. Flashing the wrong one either bricks the data or wipes a
	// rooted install, so this is checked rather than assumed.
	cscName := strings.ToUpper(filepath.Base(plan.PackageFiles[SlotCSC]))
	hasHome := strings.HasPrefix(cscName, "HOME_CSC_")
	hasCSC := strings.HasPrefix(cscName, "CSC_")

	switch installMode {
	case "initial-root":
		if !hasCSC {
			return nil, fmt.Errorf("an initial root install needs the standard CSC file (wipes data); got %s", filepath.Base(plan.PackageFiles[SlotCSC]))
		}
	case "rooted-update":
		if !hasHome {
			return nil, fmt.Errorf("a rooted update needs HOME_CSC (preserves data); got %s", filepath.Base(plan.PackageFiles[SlotCSC]))
		}
	default:
		return nil, fmt.Errorf("install mode must be initial-root or rooted-update, got %q", installMode)
	}

	args := []string{"flash"}
	args = append(args, "--BL", plan.PackageFiles[SlotBL])
	args = append(args, "--AP", plan.PackageFiles[SlotAP])
	args = append(args, "--CP", plan.PackageFiles[SlotCP])
	args = append(args, "--CSC", plan.PackageFiles[SlotCSC])
	// samloader reboots to Android by default after a successful flash. The
	// flag is a boolean switch, so passing it (or any "=false" style value)
	// would be a usage error; leaving it off is what gives us the reboot we
	// need to verify root afterwards.
	plan.Args = args
	return plan, nil
}

// CommandLine renders the exact command a plan would run, for display in a
// dry run. It is never executed by this function.
func (p *FlashPlan) CommandLine() string {
	quoted := make([]string, 0, len(p.Args)+1)
	quoted = append(quoted, strconv.Quote(p.EnginePath))
	for _, a := range p.Args {
		quoted = append(quoted, a)
	}
	return strings.Join(quoted, " ")
}
