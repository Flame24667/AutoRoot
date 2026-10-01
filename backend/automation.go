package main

import (
	"archive/tar"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Samsung packages are a TAR followed by a digest line, NOT a digest sidecar.
func embeddedMD5(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	n := int64(4096)
	if st.Size() < n {
		n = st.Size()
	}
	tail := make([]byte, n)
	if _, err = f.ReadAt(tail, st.Size()-n); err != nil {
		return "", 0, err
	}
	re := regexp.MustCompile(`(?m)([a-fA-F0-9]{32})[ \t]+[^\r\n\x00]+(?:\r?\n)?[\x00]*$`)
	match := re.FindSubmatchIndex(tail)
	if match == nil {
		return "", 0, fmt.Errorf("Samsung MD5 trailer missing")
	}
	offset := st.Size() - n + int64(match[0])
	if offset < 1024 {
		return "", 0, fmt.Errorf("MD5 trailer not at a TAR block boundary")
	}
	if offset%512 != 0 {
		// Some Samsung CSC packages append build information before the MD5.
		// The digest covers that information too. Require the observed bounded
		// format and an aligned TAR end; never silently round down the hash.
		meta := regexp.MustCompile(`Show the build information\nRBS BUILD_ID:[0-9]+\noriginal_tar_file_size:([0-9]+)\n$`).FindSubmatchIndex(tail[:match[0]])
		if meta == nil {
			return "", 0, fmt.Errorf("unaligned MD5 trailer without recognized Samsung build information")
		}
		tarSize, e := strconv.ParseInt(string(tail[meta[2]:meta[3]]), 10, 64)
		if e != nil || tarSize < 1024 || tarSize%512 != 0 || tarSize != st.Size()-n+int64(meta[0]) {
			return "", 0, fmt.Errorf("invalid original TAR size in Samsung build information")
		}
	}
	return strings.ToLower(string(tail[match[2]:match[3]])), offset, nil
}

func verifyEmbeddedMD5(path string, zipCRC ...uint32) error {
	want, n, err := embeddedMD5(path)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := md5.New()
	crc := crc32.NewIEEE()
	if _, err = io.CopyN(io.MultiWriter(h, crc), &progressReader{reader: f, total: n, detail: "MD5/ZIP CRC: " + filepath.Base(path)}, n); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return fmt.Errorf("md5 mismatch for %s", filepath.Base(path))
	}
	if len(zipCRC) > 0 {
		if _, err = io.Copy(crc, f); err != nil {
			return err
		}
		if crc.Sum32() != zipCRC[0] {
			return fmt.Errorf("extracted member does not match ZIP CRC: %s", filepath.Base(path))
		}
	}
	// Read TAR headers as well, so a digest over arbitrary bytes is not a valid package.
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}
	tr := tar.NewReader(io.NewSectionReader(f, 0, n))
	count := 0
	for {
		_, err = tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid TAR: %w", err)
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("empty TAR")
	}
	return nil
}

func deviceProperty(serial, key string) string {
	out, _, err := runAdb("-s", serial, "shell", "getprop", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Bind approval to the phone, install mode and every actual input byte.
func flashFingerprint(s *Session) (string, error) {
	if s == nil {
		return "", fmt.Errorf("session missing")
	}
	pkg, e := packageFromSession(s)
	if e != "" {
		return "", fmt.Errorf("%s", e)
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00initial-root\n", s.Serial, s.Model, s.CSC, s.DeviceBinary)
	for _, slot := range RequiredSlots {
		sum, err := FileSHA256(pkg[slot])
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", slot, pkg[slot], sum)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type catalog struct {
	Devices []struct {
		Model    string            `json:"model"`
		Firmware []catalogFirmware `json:"firmware"`
	} `json:"devices"`
}
type catalogFirmware struct {
	Region            string `json:"region"`
	PDA               string `json:"pda"`
	CSC               string `json:"csc"`
	BootloaderBinary  string `json:"bootloaderBinary"`
	SourceType        string `json:"sourceType"`
	LocalPath         string `json:"localPath"`
	URL               string `json:"url"`
	SHA256            string `json:"sha256"`
	ExpectedSizeBytes int64  `json:"expectedSizeBytes"`
}

func catalogPath() string {
	cwd, _ := os.Getwd()
	return selectCatalogPath([]string{cwd, appDir(), filepath.Dir(appDir())}, os.Getenv("AUTOROOT_DATABASE"))
}

func selectCatalogPath(roots []string, override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	for _, root := range roots {
		for _, name := range []string{"firmware-db.local.json", "firmware-db.json"} {
			p := filepath.Join(root, "setup", name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	return filepath.Join("setup", "firmware-db.json")
}

func catalogLocalPath(database, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	// A catalog normally lives in project/setup. Arbitrary override catalogs
	// resolve relative paths beside themselves instead of the shell's cwd.
	base := filepath.Dir(database)
	if strings.EqualFold(filepath.Base(base), "setup") {
		base = filepath.Dir(base)
	}
	return filepath.Clean(filepath.Join(base, filepath.FromSlash(path)))
}

func selectCatalog(data []byte, s *Session) (*catalogFirmware, error) {
	if s == nil || s.Serial == "" || len(s.DeviceBinary) != 1 {
		return nil, fmt.Errorf("authorized device session with known binary required")
	}
	var db catalog
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, err
	}
	var chosen *catalogFirmware
	for _, d := range db.Devices {
		if modelCodeOf(d.Model) != modelCodeOf(s.Model) {
			continue
		}
		for _, fw := range d.Firmware {
			if !strings.EqualFold(fw.Region, s.CSC) || fw.PDA != s.Provenance["deviceBuild"] {
				continue
			}
			if len(fw.BootloaderBinary) != 1 || binaryFromName(fw.PDA) != fw.BootloaderBinary || compareRevisions(fw.BootloaderBinary, s.DeviceBinary) < 0 {
				continue
			}
			if fw.CSC == "" || fw.CSC != s.Provenance["deviceCSCBuild"] {
				continue
			}
			if chosen != nil {
				return nil, fmt.Errorf("ambiguous database: multiple matching firmware records")
			}
			copy := fw
			chosen = &copy
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("database has no exact model/region/current-build/OMC match; refusing a guessed firmware")
	}
	return chosen, nil
}

func databasePlan(_ interface{}) (interface{}, string) {
	s := LoadSession()
	database := catalogPath()
	data, err := os.ReadFile(database)
	if err != nil {
		return nil, err.Error()
	}
	fw, err := selectCatalog(data, s)
	if err != nil {
		return nil, err.Error()
	}
	fw.LocalPath = catalogLocalPath(database, fw.LocalPath)
	found := false
	if fw.LocalPath != "" {
		st, e := os.Stat(fw.LocalPath)
		found = e == nil && !st.IsDir()
	}
	source := "unavailable"
	if found {
		source = "local"
	} else if fw.URL != "" && len(fw.SHA256) == 64 && strings.HasPrefix(fw.URL, "https://") {
		source = "download"
	}
	return map[string]interface{}{"record": fw, "source": source, "ready": source != "unavailable", "message": "Database selects firmware; it does not patch or flash the phone. Exact matching and full validation are still required."}, ""
}

func prepareDatabase(_ interface{}) (interface{}, string) {
	result, e := databasePlan(nil)
	if e != "" {
		return nil, e
	}
	plan := result.(map[string]interface{})
	fw := plan["record"].(*catalogFirmware)
	source := plan["source"].(string)
	archive := fw.LocalPath
	if source == "download" {
		s := LoadSession()
		dest := filepath.Join(deviceFirmwareDir(s.Model), fw.PDA+".zip")
		res, err := Download(DownloadRequest{URL: fw.URL, DestPath: dest, ExpectedSHA256: fw.SHA256, ExpectedSize: fw.ExpectedSizeBytes}, nil)
		if err != nil {
			return nil, err.Error()
		}
		archive = res.Path
	} else if source != "local" {
		return nil, "matching database entry has no available local file or HTTPS download with a recorded SHA-256; no download attempted"
	}
	if fw.ExpectedSizeBytes > 0 {
		st, err := os.Stat(archive)
		if err != nil {
			return nil, err.Error()
		}
		if st.Size() != fw.ExpectedSizeBytes {
			return nil, "database package size mismatch"
		}
	}
	if fw.SHA256 != "" {
		sum, err := FileSHA256(archive)
		if err != nil {
			return nil, err.Error()
		}
		if !strings.EqualFold(sum, fw.SHA256) {
			return nil, "database package SHA-256 mismatch"
		}
	}
	return adoptFirmware(map[string]interface{}{"archivePath": archive})
}

type automationJob struct {
	ID        string      `json:"id"`
	Operation string      `json:"operation"`
	Status    string      `json:"status"`
	Started   string      `json:"started"`
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	Detail    string      `json:"detail,omitempty"`
	Percent   int         `json:"percent"`
}

var jobMu sync.Mutex
var currentJob *automationJob

func updateJobProgress(detail string, done, total int64) {
	jobMu.Lock()
	defer jobMu.Unlock()
	if currentJob == nil || currentJob.Status != "running" {
		return
	}
	currentJob.Detail = detail
	if total > 0 {
		currentJob.Percent = int(done * 100 / total)
	}
}

type progressReader struct {
	reader      io.Reader
	detail      string
	done, total int64
	updated     time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.reader.Read(b)
	p.done += int64(n)
	if time.Since(p.updated) > time.Second || err == io.EOF {
		updateJobProgress(p.detail, p.done, p.total)
		p.updated = time.Now()
	}
	return n, err
}

func automationBusy() bool {
	jobMu.Lock()
	defer jobMu.Unlock()
	return currentJob != nil && currentJob.Status == "running"
}
func automationStatus() interface{} {
	jobMu.Lock()
	defer jobMu.Unlock()
	if currentJob == nil {
		return map[string]interface{}{"status": "idle"}
	}
	copy := *currentJob
	return copy
}
func startAutomationJob(payload interface{}) (interface{}, string) {
	p, _ := payload.(map[string]interface{})
	op, _ := p["operation"].(string)
	allowed := map[string]bool{"recoverEngineLaunch": true, "prepareDatabase": true, "adoptFirmware": true, "recordPatchedAP": true, "preparePatch": true, "collectPatch": true, "automateMagiskPatch": true, "approveFlash": true, "probeEngine": true, "bindDownloadDevice": true, "executeFlash": true}
	if !allowed[op] {
		return nil, "unsupported job operation (flashing is not a preparation job)"
	}
	jobMu.Lock()
	if currentJob != nil && currentJob.Status == "running" {
		jobMu.Unlock()
		return nil, "a job is already running"
	}
	currentJob = &automationJob{ID: fmt.Sprintf("job-%d", time.Now().UnixNano()), Operation: op, Status: "running", Started: time.Now().Format(time.RFC3339)}
	copy := *currentJob
	jobMu.Unlock()
	go func() {
		var result interface{}
		var e string
		defer func() {
			if r := recover(); r != nil {
				e = fmt.Sprintf("job failed: %v", r)
			}
			jobMu.Lock()
			defer jobMu.Unlock()
			currentJob.Result = result
			currentJob.Error = e
			currentJob.Status = "completed"
			if e != "" {
				currentJob.Status = "failed"
			}
		}()
		switch op {
		case "recoverEngineLaunch":
			result, e = recoverEngineLaunch(p)
		case "prepareDatabase":
			result, e = prepareDatabase(p)
		case "adoptFirmware":
			result, e = adoptFirmware(p)
		case "recordPatchedAP":
			result, e = recordPatchedAP(p)
		case "preparePatch":
			result, e = preparePatch(p)
		case "collectPatch":
			result, e = collectPatch(p)
		case "automateMagiskPatch":
			result, e = automateMagiskPatch(p)
		case "approveFlash":
			result, e = approveFlash(p)
		case "probeEngine":
			result, e = probeEngine(p)
		case "bindDownloadDevice":
			result, e = bindDownloadDevice(p)
		case "executeFlash":
			result, e = executeFlash(p)
		}
	}()
	return copy, ""
}

// Shell commands used here always select the recorded serial. Never choose an
// old random magisk_patched* result from an earlier run.
func preparePatch(_ interface{}) (interface{}, string) {
	s := LoadSession()
	if s == nil || s.Stage != StageFirmwareValid {
		return nil, "validate firmware first"
	}
	if deviceProperty(s.Serial, "ro.product.model") != s.Model {
		return nil, "recorded phone not connected"
	}
	if _, e := ensureMagiskInstalled(s.Serial); e != "" {
		return nil, e
	}
	deviceTime, _, err := runAdb("-s", s.Serial, "shell", "date", "+%s")
	started, errTime := strconv.ParseInt(strings.TrimSpace(deviceTime), 10, 64)
	if err != nil || errTime != nil || started <= 0 {
		return nil, "cannot establish phone-side patch timestamp"
	}
	remote := "/sdcard/Download/AutoRoot-" + s.Serial + "-AP.tar.md5"
	if _, stderr, err := runAdb("-s", s.Serial, "push", s.Artifact("slot-AP"), remote); err != nil {
		return nil, stderr + err.Error()
	}
	s.SetProvenance("patchStarted", fmt.Sprintf("%d", started))
	s.SetArtifact("remoteAP", remote)
	return map[string]interface{}{"ok": true, "remoteAP": remote, "requiresUser": true, "message": "In Magisk, Select and Patch this AP on this phone. Do not select Recovery Mode for the tested SM-A065F ramdisk configuration. Then collect the new output."}, ""
}

func collectPatch(payload interface{}) (interface{}, string) {
	p, _ := payload.(map[string]interface{})
	remote, _ := p["remotePath"].(string)
	if !regexp.MustCompile(`^/sdcard/Download/magisk_patched[-_][A-Za-z0-9_-]+\.tar$`).MatchString(remote) {
		return nil, "provide the exact new Magisk output path from this patch, not a wildcard"
	}
	s := LoadSession()
	if s == nil || s.Stage != StageFirmwareValid || s.Artifact("remoteAP") == "" {
		return nil, "prepare the patch on the recorded phone first"
	}
	if deviceProperty(s.Serial, "ro.product.model") != s.Model {
		return nil, "recorded phone not connected"
	}
	modified, _, err := runAdb("-s", s.Serial, "shell", "stat", "-c", "%Y", remote)
	modifiedAt, modifiedErr := strconv.ParseInt(strings.TrimSpace(modified), 10, 64)
	startedAt, startedErr := strconv.ParseInt(s.Provenance["patchStarted"], 10, 64)
	if err != nil || modifiedErr != nil || startedErr != nil || startedAt <= 0 || modifiedAt < startedAt {
		return nil, "output predates this patch or timestamp could not be checked"
	}
	dest := filepath.Join(deviceFirmwareDir(s.Model), "patched", filepath.Base(remote))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return nil, err.Error()
	}
	if _, stderr, err := runAdb("-s", s.Serial, "pull", remote, dest); err != nil {
		return nil, stderr + err.Error()
	}
	localHash, err := FileSHA256(dest)
	if err != nil {
		return nil, err.Error()
	}
	remoteHash, stderr, err := runAdb("-s", s.Serial, "shell", "sha256sum", remote)
	fields := strings.Fields(remoteHash)
	if err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], localHash) {
		return nil, "phone/PC patch checksum mismatch: " + stderr
	}
	return recordPatchedAP(map[string]interface{}{"path": dest})
}

// Structural check, not a claim that arbitrary different bytes prove Magisk.
// Patch provenance is supplied by preparePatch/collectPatch on the same phone.
func inspectPatchedAP(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	seen := map[string]bool{}
	boot := false
	vbmeta := false
	for {
		hdr, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return fmt.Errorf("invalid patched TAR: %w", e)
		}
		name := hdr.Name
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
			return fmt.Errorf("unsafe TAR member")
		}
		if seen[name] {
			return fmt.Errorf("duplicate patched TAR member: %s", name)
		}
		seen[name] = true
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			return fmt.Errorf("non-regular TAR member")
		}
		if name == "boot.img" || name == "init_boot.img" {
			head := make([]byte, 8)
			if _, e = io.ReadFull(tr, head); e != nil || string(head) != "ANDROID!" {
				return fmt.Errorf("patched boot has invalid Android header")
			}
			boot = true
		}
		if name == "vbmeta.img" {
			head := make([]byte, 4)
			if _, e = io.ReadFull(tr, head); e != nil || string(head) != "AVB0" {
				return fmt.Errorf("patched vbmeta has invalid AVB header")
			}
			vbmeta = true
		}
	}
	if !boot || !vbmeta || !seen["super.img.lz4"] {
		return fmt.Errorf("patched AP missing boot, vbmeta or super for SM-A065F")
	}
	return nil
}

// Reserved for command timeouts shared by engine checks.
func engineContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 45*time.Second)
}

func verifiedDownloadIdentity(s *Session) error {
	res, err := waitDownloadIdentity(s.Model)
	if err != nil {
		return err
	}
	return validateDownloadIdentity(s, res)
}

func validateDownloadIdentity(s *Session, res map[string]interface{}) error {
	if s == nil {
		return fmt.Errorf("session missing")
	}
	model, _ := res["modelName"].(string)
	serial, _ := res["serialNumber"].(string)
	expected := s.Serial
	if binding := loadDownloadBinding(s); binding != nil {
		expected = binding.DownloadSerial
	}
	if model != s.Model || serial == "" || serial != expected {
		return fmt.Errorf("Download Mode identity not proven: model=%q serial=%q; expected %s/%s. Stop; do not bypass this gate", model, serial, s.Model, s.Serial)
	}
	return nil
}

// This probe changes boot mode only with explicit consent. PIT is READ from
// the phone, never written to it. An empty diagnostic response is not proof.
func probeEngine(payload interface{}) (interface{}, string) {
	p, _ := payload.(map[string]interface{})
	confirmed, _ := p["confirmReboot"].(bool)
	if !confirmed {
		return nil, "explicit Download Mode reboot confirmation required"
	}
	s := LoadSession()
	if s == nil {
		return nil, "start a session first"
	}
	if _, err := resolveSamloader(); err != nil {
		return nil, err.Error()
	}
	if deviceProperty(s.Serial, "ro.product.model") != s.Model {
		return nil, "recorded Android device not connected"
	}
	if _, e := rebootToDownloadMode(s.Serial); e != "" {
		return nil, e
	}
	time.Sleep(8 * time.Second)
	res, err := waitDownloadIdentity(s.Model)
	if err != nil {
		return nil, err.Error()
	}
	if model, _ := res["modelName"].(string); model == s.Model && loadDownloadBinding(s) == nil {
		serial, _ := res["serialNumber"].(string)
		if serial != "" && serial != s.Serial {
			return map[string]interface{}{"ok": true, "needsBinding": true, "model": model, "adbSerial": s.Serial, "downloadSerial": serial, "message": "Download Mode uses a different identifier. Confirm only this phone is connected and explicitly bind the displayed identifier before proceeding."}, ""
		}
	}
	if err := validateDownloadIdentity(s, res); err != nil {
		return nil, err.Error() + "; phone may remain in Download Mode"
	}
	pit := filepath.Join(deviceFirmwareDir(s.Model), "probe", s.Serial+".pit")
	out, err := DumpPIT(pit)
	if err != nil {
		return nil, "engine PIT read failed: " + err.Error() + " " + out
	}
	st, err := os.Stat(pit)
	if err != nil || st.Size() < 28 {
		return nil, "PIT read returned empty/invalid data"
	}
	f, err := os.Open(pit)
	if err != nil {
		return nil, err.Error()
	}
	header := make([]byte, 4)
	_, err = io.ReadFull(f, header)
	f.Close()
	if err != nil || hex.EncodeToString(header) != "76983412" {
		return nil, "PIT magic is invalid"
	}
	s.SetArtifact("enginePIT", pit)
	s.SetProvenance("engineProbeSerial", s.Serial)
	s.SetProvenance("engineProbeTime", time.Now().Format(time.RFC3339))
	sum, err := FileSHA256(pit)
	if err != nil {
		return nil, err.Error()
	}
	s.SetProvenance("enginePITSHA256", sum)
	return map[string]interface{}{"ok": true, "flashed": false, "pit": pit, "message": "Identity and PIT read succeeded. Phone remains in Download Mode. Restart it manually before preparation/preflight."}, ""
}

// Windows USB enumeration can lag behind the reboot. Retry only read-only
// detection with a bounded deadline, never reboot or flash as a retry.
func waitDownloadIdentity(expectedModel string) (map[string]interface{}, error) {
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		info, err := DeviceDetect(false, true)
		if err == nil {
			model, _ := info["modelName"].(string)
			serial, _ := info["serialNumber"].(string)
			if model != "" && model != expectedModel {
				return nil, fmt.Errorf("unexpected Download Mode model %q", model)
			}
			if model == expectedModel && serial != "" {
				return info, nil
			}
			lastErr = fmt.Errorf("USB identity is not available yet")
		} else {
			lastErr = err
		}
		updateJobProgress("Wait for Windows Download Mode USB identity", 0, 1)
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("Download Mode identification timed out: %v", lastErr)
}

type downloadBinding struct {
	ADBSerial      string `json:"adbSerial"`
	DownloadSerial string `json:"downloadSerial"`
	Model          string `json:"model"`
	ConfirmedAt    string `json:"confirmedAt"`
	PIT            string `json:"pit"`
	PITSHA256      string `json:"pitSHA256"`
}

func bindingPath(s *Session) string {
	return filepath.Join(writableStateDir(), "binding-"+sanitizeModelDir(s.Serial)+".json")
}
func loadDownloadBinding(s *Session) *downloadBinding {
	if s == nil {
		return nil
	}
	data, err := os.ReadFile(bindingPath(s))
	if err != nil {
		return nil
	}
	var b downloadBinding
	if json.Unmarshal(data, &b) != nil || b.ADBSerial != s.Serial || b.Model != s.Model || b.DownloadSerial == "" || b.ConfirmedAt == "" {
		return nil
	}
	return &b
}
func bindDownloadDevice(payload interface{}) (interface{}, string) {
	p, _ := payload.(map[string]interface{})
	confirmed, _ := p["confirmOnlyDevice"].(bool)
	wanted, _ := p["downloadSerial"].(string)
	if !confirmed || wanted == "" {
		return nil, "operator must explicitly confirm the displayed Download Mode identity and that only this phone is connected"
	}
	s := LoadSession()
	if s == nil {
		return nil, "session missing"
	}
	info, err := DeviceDetect(false, true)
	if err != nil {
		return nil, err.Error()
	}
	model, _ := info["modelName"].(string)
	serial, _ := info["serialNumber"].(string)
	if model != s.Model || serial != wanted {
		return nil, "live Download Mode identity does not match operator confirmation"
	}
	pit := filepath.Join(deviceFirmwareDir(s.Model), "probe", s.Serial+".pit")
	out, err := DumpPIT(pit)
	if err != nil {
		return nil, "PIT read failed: " + err.Error() + out
	}
	f, err := os.Open(pit)
	if err != nil {
		return nil, err.Error()
	}
	header := make([]byte, 4)
	_, err = io.ReadFull(f, header)
	f.Close()
	if err != nil || hex.EncodeToString(header) != "76983412" {
		return nil, "invalid PIT magic"
	}
	sum, err := FileSHA256(pit)
	if err != nil {
		return nil, err.Error()
	}
	b := downloadBinding{ADBSerial: s.Serial, DownloadSerial: serial, Model: model, ConfirmedAt: time.Now().Format(time.RFC3339), PIT: pit, PITSHA256: sum}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err.Error()
	}
	path := bindingPath(s)
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err.Error()
	}
	if err = os.WriteFile(path+".tmp", data, 0600); err != nil {
		return nil, err.Error()
	}
	if err = os.Rename(path+".tmp", path); err != nil {
		return nil, err.Error()
	}
	return map[string]interface{}{"ok": true, "flashed": false, "binding": b, "message": "Operator-confirmed identity pairing and read-only PIT probe recorded. Restart phone manually; this is NOT permission to flash."}, ""
}

func requireBoundApproval(s *Session) error {
	if s == nil {
		return fmt.Errorf("no session")
	}
	if err := s.RequireApproval(); err != nil {
		return err
	}
	if s.Stage != StageApprovedFlash {
		return fmt.Errorf("flash requires freshly approved stage; an interrupted flash is never retried automatically")
	}
	stamp, err := time.Parse(time.RFC3339, s.Approval.GrantedAt)
	if err != nil || time.Since(stamp) > 30*time.Minute || time.Until(stamp) > time.Minute {
		return fmt.Errorf("approval expired; recheck and approve again")
	}
	sum, err := flashFingerprint(s)
	if err != nil {
		return err
	}
	if sum != s.Approval.Fingerprint {
		return fmt.Errorf("files/phone changed after approval; refusing flash")
	}
	return nil
}

func executeFlash(payload interface{}) (interface{}, string) {
	p, _ := payload.(map[string]interface{})
	wipe, _ := p["confirmWipe"].(bool)
	if !wipe {
		return nil, "explicit final flash/wipe confirmation required"
	}
	s := LoadSession()
	if err := requireBoundApproval(s); err != nil {
		return nil, err.Error()
	}
	binding := loadDownloadBinding(s)
	if binding == nil {
		return nil, "operator-confirmed identity binding and PIT probe required"
	}
	pitSum, err := FileSHA256(binding.PIT)
	if err != nil || pitSum != binding.PITSHA256 {
		return nil, "recorded engine probe evidence missing or changed"
	}
	report := RunPreflight(s)
	if !report.OK {
		return nil, "preflight failed: " + strings.Join(report.Blockers, "; ")
	}
	pkg, e := packageFromSession(s)
	if e != "" {
		return nil, e
	}
	plan, err := BuildFlashPlan(pkg, "initial-root", s)
	if err != nil {
		return nil, err.Error()
	}
	if _, e = rebootToDownloadMode(s.Serial); e != "" {
		return nil, e
	}
	time.Sleep(8 * time.Second)
	if err = verifiedDownloadIdentity(s); err != nil {
		return nil, err.Error()
	}
	if err = s.Advance(StageDownloadMode); err != nil {
		return nil, err.Error()
	}
	if err = s.Advance(StageFlashing); err != nil {
		return nil, err.Error()
	}
	// Do not kill a flash on an arbitrary timeout. UI/app closure is inhibited.
	cmd := exec.Command(plan.EnginePath, plan.Args...)
	logPath := filepath.Join(logDir(), "flash-"+s.Serial+"-"+time.Now().Format("20060102-150405")+".log")
	log, err := os.Create(logPath)
	if err != nil {
		return nil, err.Error()
	}
	cmd.Stdout = log
	cmd.Stderr = log
	s.Provenance["flashLaunchState"] = "unknown"
	s.Provenance["flashEngine"] = plan.EnginePath
	s.Provenance["flashLog"] = logPath
	if err = s.Save(); err != nil {
		log.Close()
		return nil, err.Error()
	}
	err = cmd.Start()
	if err != nil {
		s.Provenance["flashLaunchState"] = "not-started"
	} else {
		s.Provenance["flashLaunchState"] = "started"
		// If saving fails after Start, still wait without killing the engine.
		if saveErr := s.Save(); saveErr != nil {
			fmt.Fprintln(log, "WARNING: failed to persist process-start evidence:", saveErr)
		}
		err = cmd.Wait()
	}
	log.Close()
	if err != nil {
		_ = s.Fail("flash failed: " + err.Error() + "; no automatic retry")
		return nil, "flashing failed; inspect phone and " + logPath + ". Do not retry automatically"
	}
	if err = s.Advance(StageWaitingBoot); err != nil {
		return nil, err.Error()
	}
	s.Approval = nil
	if err = s.Save(); err != nil {
		return nil, err.Error()
	}
	return map[string]interface{}{"ok": true, "rooted": false, "stage": s.Stage, "log": logPath, "message": "Engine completed without error. Root is NOT yet verified. Complete phone setup, restore USB debugging, install/open Magisk and complete additional setup, then verify uid=0."}, ""
}
