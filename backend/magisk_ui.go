package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// UI nodes are observed from the live Android hierarchy before every tap.
// This adapter is scoped to on-phone file patching, never Direct Install,
// reboot, bootloader confirmation, or flashing.
type phoneUINode struct {
	Text        string        `xml:"text,attr"`
	ID          string        `xml:"resource-id,attr"`
	Package     string        `xml:"package,attr"`
	Description string        `xml:"content-desc,attr"`
	Class       string        `xml:"class,attr"`
	Checked     string        `xml:"checked,attr"`
	Clickable   string        `xml:"clickable,attr"`
	Enabled     string        `xml:"enabled,attr"`
	Bounds      string        `xml:"bounds,attr"`
	Children    []phoneUINode `xml:"node"`
}

// Android charging-source bits: AC=1, USB=2, wireless=4, dock=8.
// Preserve the full supported mask, including dock, instead of truncating to 7.
func parseScreenAwakePreference(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 || value > 15 {
		return 0, fmt.Errorf("invalid screen-awake charging-source mask %q", strings.TrimSpace(raw))
	}
	return value, nil
}

func parsePhoneUI(data string) ([]phoneUINode, error) {
	var root struct {
		Nodes []phoneUINode `xml:"node"`
	}
	start := strings.Index(data, "<?xml")
	if start < 0 {
		start = strings.Index(data, "<hierarchy")
	}
	if start < 0 {
		return nil, fmt.Errorf("Android UI XML missing")
	}
	if err := xml.Unmarshal([]byte(data[start:]), &root); err != nil {
		return nil, err
	}
	var nodes []phoneUINode
	var walk func(phoneUINode)
	walk = func(n phoneUINode) {
		nodes = append(nodes, n)
		for _, child := range n.Children {
			walk(child)
		}
	}
	for _, n := range root.Nodes {
		walk(n)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("empty Android UI")
	}
	return nodes, nil
}

func nodeCenter(n phoneUINode) (int, int, error) {
	m := regexp.MustCompile(`^\[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\]$`).FindStringSubmatch(n.Bounds)
	if len(m) != 5 {
		return 0, 0, fmt.Errorf("invalid observed bounds")
	}
	v := make([]int, 4)
	for i := range v {
		var err error
		v[i], err = strconv.Atoi(m[i+1])
		if err != nil {
			return 0, 0, err
		}
	}
	if v[2] <= v[0] || v[3] <= v[1] {
		return 0, 0, fmt.Errorf("empty observed bounds")
	}
	return (v[0] + v[2]) / 2, (v[1] + v[3]) / 2, nil
}

func singleUINode(nodes []phoneUINode, packageName, label string) (phoneUINode, error) {
	var found []phoneUINode
	for _, n := range nodes {
		if n.Package == packageName && n.Enabled == "true" && (n.Text == label || n.Description == label) {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		return phoneUINode{}, fmt.Errorf("expected one %q control in %s; found %d", label, packageName, len(found))
	}
	if _, _, err := nodeCenter(found[0]); err != nil {
		return phoneUINode{}, err
	}
	return found[0], nil
}

func magiskInstallNode(nodes []phoneUINode) (phoneUINode, error) {
	var title, app phoneUINode
	for _, n := range nodes {
		if n.ID == "com.topjohnwu.magisk:id/home_magisk_title" {
			title = n
		}
		if n.ID == "com.topjohnwu.magisk:id/home_manager_title" {
			app = n
		}
	}
	_, magiskY, err := nodeCenter(title)
	if err != nil {
		return phoneUINode{}, fmt.Errorf("recognized Magisk home required")
	}
	_, appY, err := nodeCenter(app)
	if err != nil || appY <= magiskY {
		return phoneUINode{}, fmt.Errorf("Magisk/App cards not distinguished")
	}
	var found []phoneUINode
	for _, n := range nodes {
		if n.Package != "com.topjohnwu.magisk" || n.Text != "Install" || n.Clickable != "true" || n.Enabled != "true" {
			continue
		}
		_, y, e := nodeCenter(n)
		if e == nil && y < appY && y >= magiskY-100 {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		return phoneUINode{}, fmt.Errorf("Magisk install control ambiguous")
	}
	return found[0], nil
}

func readPhoneUI(serial string) ([]phoneUINode, error) {
	const remote = "/sdcard/Download/autoroot-ui.xml"
	if dump, stderr, err := runAdb("-s", serial, "shell", "uiautomator", "dump", remote); err != nil || !strings.Contains(dump, "dumped to:") {
		if err == nil {
			err = fmt.Errorf("fresh hierarchy not written: %s", dump)
		}
		return nil, fmt.Errorf("UI dump: %s %w", stderr, err)
	}
	out, _, err := runAdb("-s", serial, "shell", "cat", remote)
	if err != nil {
		return nil, err
	}
	return parsePhoneUI(out)
}

func tapPhoneNode(serial string, n phoneUINode) error {
	x, y, err := nodeCenter(n)
	if err != nil {
		return err
	}
	if n.Enabled != "true" {
		return fmt.Errorf("disabled UI control")
	}
	_, stderr, err := runAdb("-s", serial, "shell", "input", "tap", strconv.Itoa(x), strconv.Itoa(y))
	if err != nil {
		return fmt.Errorf("tap: %s %w", stderr, err)
	}
	return nil
}

const magiskUIPackage = "com.topjohnwu.magisk"
const pickerUIPackage = "com.google.android.documentsui"

func uiHas(nodes []phoneUINode, packageName, text string) bool {
	for _, n := range nodes {
		if n.Package == packageName && n.Text == text {
			return true
		}
	}
	return false
}

func requireRamdiskHome(nodes []phoneUINode) error {
	if !uiHas(nodes, magiskUIPackage, "N/A") {
		return fmt.Errorf("unrooted Magisk home not proven")
	}
	label, err := singleUINode(nodes, magiskUIPackage, "Ramdisk")
	if err != nil {
		return err
	}
	_, labelY, err := nodeCenter(label)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if n.Package == magiskUIPackage && n.Text == "Yes" {
			_, y, e := nodeCenter(n)
			if e == nil && y == labelY {
				return nil
			}
		}
	}
	return fmt.Errorf("Ramdisk Yes not proven; no automatic patch for recovery-mode devices")
}

func tapObservedLabel(serial, pkg, label string) error {
	nodes, err := readPhoneUI(serial)
	if err != nil {
		return err
	}
	n, err := singleUINode(nodes, pkg, label)
	if err != nil {
		return err
	}
	return tapPhoneNode(serial, n)
}

func tapObservedID(serial, pkg, id string) error {
	nodes, err := readPhoneUI(serial)
	if err != nil {
		return err
	}
	var found []phoneUINode
	for _, n := range nodes {
		if n.Package == pkg && n.ID == id && n.Enabled == "true" {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		return fmt.Errorf("expected one observed %s control", id)
	}
	return tapPhoneNode(serial, found[0])
}

func selectPreparedAP(serial, remote string) error {
	if err := tapObservedLabel(serial, pickerUIPackage, "Show roots"); err != nil {
		return err
	}
	if err := tapPickerTitle(serial, "Galaxy A06"); err != nil {
		return err
	}
	if err := tapPickerTitle(serial, "Download"); err != nil {
		return err
	}
	nodes, err := readPhoneUI(serial)
	if err != nil {
		return err
	}
	// Require observed breadcrumbs, not merely a similarly named search result.
	seenRoot, seenDownload := false, false
	for _, n := range nodes {
		if n.ID == pickerUIPackage+":id/breadcrumb_text" {
			seenRoot = seenRoot || n.Text == "Galaxy A06"
			seenDownload = seenDownload || n.Text == "Download"
		}
	}
	if !seenRoot || !seenDownload {
		return fmt.Errorf("internal-storage Download folder not proven")
	}
	var found []phoneUINode
	for _, n := range nodes {
		if n.Package == pickerUIPackage && n.ID == "android:id/title" && n.Text == filepath.Base(remote) && n.Enabled == "true" {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		return fmt.Errorf("prepared AP missing or ambiguous in live file picker")
	}
	return tapPhoneNode(serial, found[0])
}

func tapPickerTitle(serial, label string) error {
	nodes, err := readPhoneUI(serial)
	if err != nil {
		return err
	}
	var found []phoneUINode
	for _, n := range nodes {
		if n.Package == pickerUIPackage && n.ID == "android:id/title" && n.Text == label && n.Enabled == "true" {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		return fmt.Errorf("picker location %q ambiguous", label)
	}
	return tapPhoneNode(serial, found[0])
}

func patchOutputFromUI(nodes []phoneUINode) (string, error) {
	var lines []string
	for _, n := range nodes {
		if n.Package == magiskUIPackage && n.Text != "" {
			lines = append(lines, n.Text)
		}
	}
	log := strings.Join(lines, "\n")
	if !strings.Contains(log, "All done!") {
		return "", fmt.Errorf("patch completion not proven in Magisk output")
	}
	re := regexp.MustCompile(`/(?:storage/emulated/0|sdcard)/Download/(magisk_patched[-_][A-Za-z0-9_-]+\.tar)`)
	matches := re.FindAllStringSubmatch(log, -1)
	names := map[string]bool{}
	for _, m := range matches {
		names[m[1]] = true
	}
	if len(names) != 1 {
		return "", fmt.Errorf("exact patched TAR output path missing or ambiguous")
	}
	for name := range names {
		return "/sdcard/Download/" + name, nil
	}
	return "", fmt.Errorf("patch output missing")
}

// Only file patching is automated here. No root-shell install, reboot, flash,
// security-dialog acceptance or lock-screen input is performed.
func automateMagiskPatch(_ interface{}) (interface{}, string) {
	s := LoadSession()
	if s == nil || s.Stage != StageFirmwareValid {
		return nil, "validated firmware session required"
	}
	if s.Model != "SM-A065F" {
		return nil, "UI adapter is tested only for SM-A065F"
	}
	remote := s.Artifact("remoteAP")
	if remote != "/sdcard/Download/AutoRoot-"+s.Serial+"-AP.tar.md5" {
		return nil, "prepare the exact AP on this phone first"
	}
	if deviceProperty(s.Serial, "ro.product.model") != s.Model || deviceProperty(s.Serial, "ro.build.PDA") != s.Provenance["deviceBuild"] || deviceProperty(s.Serial, "ro.omc.build.version") != s.Provenance["deviceCSCBuild"] {
		return nil, "recorded device/build changed"
	}
	version, _, err := runAdb("-s", s.Serial, "shell", "dumpsys", "package", magiskUIPackage)
	if err != nil || !regexp.MustCompile(`\bversionCode=30700\b`).MatchString(version) {
		return nil, "UI adapter requires verified installed Magisk 30.7 (30700)"
	}
	if state := s.Provenance["uiPatchState"]; state == "failed" {
		return nil, "prior UI patch was interrupted/failed; inspect phone, no automatic retry"
	}
	oldAwake, _, err := runAdb("-s", s.Serial, "shell", "settings", "get", "global", "stay_on_while_plugged_in")
	awake, errNumber := parseScreenAwakePreference(oldAwake)
	if err != nil {
		return nil, "cannot read screen-awake preference: " + err.Error()
	}
	if errNumber != nil {
		return nil, "cannot preserve screen-awake preference: " + errNumber.Error()
	}
	if _, _, err := runAdb("-s", s.Serial, "shell", "svc", "power", "stayon", "usb"); err != nil {
		return nil, err.Error()
	}
	defer runAdb("-s", s.Serial, "shell", "settings", "put", "global", "stay_on_while_plugged_in", strconv.Itoa(awake))
	if s.Provenance["uiPatchState"] != "running" && s.Provenance["uiPatchState"] != "completed" {
		st, err := osStatAP(s.Artifact("slot-AP"))
		if err != nil {
			return nil, err.Error()
		}
		remoteSize, _, e := runAdb("-s", s.Serial, "shell", "stat", "-c", "%s", remote)
		if e != nil || strings.TrimSpace(remoteSize) != strconv.FormatInt(st, 10) {
			return nil, "prepared AP transfer is incomplete"
		}
		// Return only from recognized picker/method screens to the observed home.
		for i := 0; i < 4; i++ {
			nodes, e := readPhoneUI(s.Serial)
			if e != nil {
				return nil, e.Error()
			}
			if _, e = magiskInstallNode(nodes); e == nil {
				break
			}
			known := false
			for _, n := range nodes {
				known = known || n.Package == pickerUIPackage || n.ID == magiskUIPackage+":id/method_patch"
			}
			if !known {
				return nil, "unlock phone and open Magisk Home; unknown UI is never tapped"
			}
			if _, _, e = runAdb("-s", s.Serial, "shell", "input", "keyevent", "KEYCODE_BACK"); e != nil {
				return nil, e.Error()
			}
		}
		nodes, e := readPhoneUI(s.Serial)
		if e != nil {
			return nil, e.Error()
		}
		if e = requireRamdiskHome(nodes); e != nil {
			return nil, e.Error()
		}
		n, e := magiskInstallNode(nodes)
		if e != nil {
			return nil, e.Error()
		}
		updateJobProgress("Select Magisk patch-file method", 0, 1)
		if e = tapPhoneNode(s.Serial, n); e != nil {
			return nil, e.Error()
		}
		if e = tapObservedID(s.Serial, magiskUIPackage, magiskUIPackage+":id/method_patch"); e != nil {
			return nil, e.Error()
		}
		if e = selectPreparedAP(s.Serial, remote); e != nil {
			return nil, e.Error()
		}
		nodes, e = readPhoneUI(s.Serial)
		if e != nil {
			return nil, e.Error()
		}
		selected := false
		for _, n := range nodes {
			if n.ID == magiskUIPackage+":id/method_patch" && n.Checked == "true" {
				selected = true
			}
			if strings.Contains(strings.ToLower(n.Text), "recovery") && n.Checked == "true" {
				return nil, "unexpected checked Recovery Mode; stopping"
			}
		}
		if !selected {
			return nil, "patch-file method not selected"
		}
		goNode, e := singleUINode(nodes, magiskUIPackage, "LET'S GO")
		if e != nil {
			return nil, e.Error()
		}
		before, _, e := runAdb("-s", s.Serial, "shell", "ls", "-1", "/sdcard/Download")
		if e != nil {
			return nil, e.Error()
		}
		s.SetProvenance("uiPatchExistingOutputs", before)
		stamp, _, e := runAdb("-s", s.Serial, "shell", "date", "+%s")
		if e != nil {
			return nil, e.Error()
		}
		if epoch, e := strconv.ParseInt(strings.TrimSpace(stamp), 10, 64); e != nil || epoch <= 0 {
			return nil, "invalid phone-side timestamp"
		}
		s.SetProvenance("patchStarted", strings.TrimSpace(stamp))
		// Persist BEFORE tapping start: an interrupted controller cannot retry.
		s.SetProvenance("uiPatchState", "running")
		if e = tapPhoneNode(s.Serial, goNode); e != nil {
			return nil, e.Error()
		}
	}
	output := s.Provenance["uiPatchOutput"]
	if output == "" {
		deadline := time.Now().Add(30 * time.Minute)
		for time.Now().Before(deadline) {
			updateJobProgress("Magisk is patching AP on phone; no partitions flashed", 0, 1)
			nodes, e := readPhoneUI(s.Serial)
			if e != nil {
				return nil, "UI observation interrupted; patch may still be running: " + e.Error()
			}
			output, e = patchOutputFromUI(nodes)
			if e == nil {
				for _, old := range strings.Fields(s.Provenance["uiPatchExistingOutputs"]) {
					if old == filepath.Base(output) {
						s.SetProvenance("uiPatchState", "failed")
						return nil, "output existed before this patch; refusing stale result"
					}
				}
				s.SetProvenance("uiPatchOutput", output)
				s.SetProvenance("uiPatchState", "completed")
				break
			}
			known := false
			for _, n := range nodes {
				if n.Package == magiskUIPackage {
					known = true
				}
				if n.Package == magiskUIPackage && (strings.Contains(n.Text, "Installation failed") || strings.Contains(n.Text, "! Unable to") || strings.Contains(n.Text, "! Invalid")) {
					s.SetProvenance("uiPatchState", "failed")
					return nil, "Magisk reported a patch failure; no automatic retry"
				}
			}
			if !known {
				return nil, "Magisk output screen unavailable; no automatic retry"
			}
			time.Sleep(5 * time.Second)
		}
		if output == "" {
			return nil, "patch observation timed out; do not retry automatically or stop a working phone patch"
		}
	}
	updateJobProgress("Pull and verify the exact new Magisk output", 0, 1)
	return collectPatch(map[string]interface{}{"remotePath": output})
}

func osStatAP(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if st.IsDir() || st.Size() <= 0 {
		return 0, fmt.Errorf("missing AP")
	}
	return st.Size(), nil
}
