package main

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// A Samsung firmware package is named like:
//   AP_A065FXXS4AYE2_A065FOLE4AYE2_26021ABCDEFGOP_20240401_000000.8102.zip
// and contains AP_/BL_/CP_/CSC_ .tar.md5 members. The model code and the
// bootloader binary revision are both encoded in the filename, which is the
// only pre-download evidence we have that a package matches the phone.

// A Samsung build segment looks like "A065FXXS4AYE2":
//
//	A065F  model code
//	XX    firmware branch marker
//	S     build type (S=system, U=user, B=bootloader)
//	4     bootloader binary revision
//	AYE2  build year + revision
//
// The regex only answers "is this token a build segment at all". Go's RE2
// engine has no lookahead, so the model/binary split is done by hand in
// splitBuildSegment below, which is both exact and easier to audit.
var (
	samsungBuildSegment = regexp.MustCompile(`(?i)^[A-Z]\d{3}[A-Z][A-Z0-9]{0,3}(XX|UU|UB|UA)[A-Z][0-9A-Z][A-Z0-9]|^[A-Z]\d{3}[A-Z]OLE[0-9A-Z][A-Z0-9]{4}$`)

	// A bare model code: a letter, three digits and a letter, e.g. "A065F" or
	// the regional variant "A065FD". Anchored so splitBuildSegment can test
	// candidate prefixes of fixed length.
	samsungModelCodeRe = regexp.MustCompile(`(?i)^[A-Z]\d{3}[A-Z][A-Z0-9]{0,2}$`)

	// Multi-CSC packages embed the supported sales codes in the name, e.g.
	// HOME_CSC_A065FXXS4AYE2_..._MTK_XID_BTU_...
	samsungCSCListRe = regexp.MustCompile(`_(?:[A-Z]{3}_)+([A-Z]{3})(?:_|$)`)
)

// firmwareBranchMarkers are the two-letter markers Samsung places between the
// model code and the bootloader binary revision in a build segment.
var firmwareBranchMarkers = []string{"XX", "UU", "UB", "UA"}

// minArchiveBytes rejects captures that are far too small to be a real
// firmware package. A Samsung ZIP is ~5.8 GB; anything under a megabyte is
// essentially always an error page or a stub. It is a variable so tests can
// exercise the surrounding logic with small fixtures.
var minArchiveBytes int64 = 1_000_000

// Slot identifies one of the four mandatory Samsung firmware partitions.
type Slot string

const (
	SlotAP  Slot = "AP"
	SlotBL  Slot = "BL"
	SlotCP  Slot = "CP"
	SlotCSC Slot = "CSC"
)

// RequiredSlots is the set a package must provide. The first Samsung Magisk
// install needs a real CSC (it performs the data wipe); a later update on an
// already-rooted phone must use HOME_CSC so userdata survives.
var RequiredSlots = []Slot{SlotAP, SlotBL, SlotCP, SlotCSC}

// FirmwareProfile is the exact device the package must match.
type FirmwareProfile struct {
	Model        string
	CSC          string
	CSCBuild     string // exact installed OMC build, not a guessed multi-CSC mapping
	DeviceBinary string // current bootloader binary revision, e.g. "4"
}

// SlotFile is one validated member of a firmware package.
type SlotFile struct {
	Slot     Slot
	Path     string
	FileName string
	Size     int64
	MD5      string // internal checksum declared by the .tar.md5 sibling
}

// FirmwareValidation is the full verdict on a candidate package.
type FirmwareValidation struct {
	OK           bool
	Errors       []string
	Warnings     []string
	ModelCode    string
	CSCInPackage string
	PkgBinary    string
	Sizes        map[Slot]SlotFile
}

func (v *FirmwareValidation) fail(format string, args ...interface{}) {
	v.Errors = append(v.Errors, fmt.Sprintf(format, args...))
}

func (v *FirmwareValidation) warn(format string, args ...interface{}) {
	v.Warnings = append(v.Warnings, fmt.Sprintf(format, args...))
}

// modelCodeOf strips a leading SM- and upper-cases a Samsung model string.
func modelCodeOf(model string) string {
	m := strings.ToUpper(strings.TrimSpace(model))
	m = strings.TrimPrefix(m, "SM-")
	return m
}

// samsungBuildSegments splits a Samsung filename into its underscore tokens and
// returns those that look like firmware build segments ("A065FXXS4AYE2").
func samsungBuildSegments(name string) []string {
	base := strings.ToUpper(strings.TrimSuffix(filepath.Base(name), ".ZIP"))
	var out []string
	for _, token := range strings.Split(base, "_") {
		if samsungBuildSegment.MatchString(token) {
			out = append(out, token)
		}
	}
	return out
}

// splitBuildSegment separates "A065FXXS4AYE2" into its model code ("A065F") and
// bootloader binary revision ("4"). The model code is a letter, three digits
// and a letter (optionally followed by up to three regional characters); the
// revision is the single character following the two-letter branch marker.
func splitBuildSegment(token string) (model, binary string) {
	token = strings.ToUpper(token)
	if len(token) == 13 && token[5:8] == "OLE" {
		return token[:5], string(token[8])
	}
	if !samsungBuildSegment.MatchString(token) {
		return "", ""
	}

	// Longest model first: "A065FD" is a valid regional variant of "A065F", and
	// greedily taking the shorter form would split at the wrong offset.
	for modelLen := 7; modelLen >= 5; modelLen-- {
		if len(token) < modelLen {
			continue
		}
		candidate := token[:modelLen]
		if !samsungModelCodeRe.MatchString(candidate) {
			continue
		}
		rest := token[modelLen:]
		for _, marker := range firmwareBranchMarkers {
			// rest = branch(2) + buildType(1) + binary(1) + ...
			if len(rest) >= 4 && strings.HasPrefix(rest, marker) {
				return candidate, string(rest[3])
			}
		}
	}
	return "", ""
}

// samsungModelFromName pulls the model code out of a Samsung filename, e.g.
// "AP_A065FXXS4AYE2_..." -> "A065F".
func samsungModelFromName(name string) string {
	segments := samsungBuildSegments(name)
	if len(segments) == 0 {
		return ""
	}
	model, _ := splitBuildSegment(segments[0])
	return model
}

// binaryFromName returns the bootloader binary revision encoded in a Samsung
// filename, e.g. "AP_A065FXXS4AYE2_..." -> "4". Returns "" when absent.
func binaryFromName(name string) string {
	segments := samsungBuildSegments(name)
	if len(segments) == 0 {
		return ""
	}
	_, binary := splitBuildSegment(segments[0])
	return binary
}

// base36Value converts a single alphanumeric revision char to its base-36
// numeric value so revisions can be ordered (4 < 5 < 6 < 7 < 8 < 9 < A ...).
func base36Value(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10, true
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10, true
	}
	return 0, false
}

// compareRevisions returns -1/0/1 for a<b, a==b, a>b.
func compareRevisions(a, b string) int {
	av, aok := base36Value(a[0])
	bv, bok := base36Value(b[0])
	if !aok || !bok {
		return 0
	}
	switch {
	case av < bv:
		return -1
	case av > bv:
		return 1
	}
	return 0
}

// isFirmwareArchive reports whether the path looks like a Samsung firmware
// ZIP by extension and name. It is a cheap pre-filter, never proof of validity.
func isFirmwareArchive(path string) bool {
	lower := strings.ToLower(filepath.Base(path))
	if !strings.HasSuffix(lower, ".zip") {
		return false
	}
	return strings.HasPrefix(upperBase(path), "AP_") ||
		strings.Contains(upperBase(path), "_AP_") ||
		strings.HasPrefix(strings.TrimPrefix(upperBase(path), "HOME_CSC_"), "AP_")
}

func upperBase(path string) string { return strings.ToUpper(filepath.Base(path)) }

// looksLikeHTMLError rejects a download that captured a login/consent page
// instead of a firmware ZIP. The most reliable signal is the extension: a real
// package is always a ZIP, so a .zip that opens as HTML is a bad capture.
func looksLikeHTMLError(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && n == 0 {
		return false
	}
	head = head[:n]
	lower := strings.ToLower(string(head))
	return strings.Contains(lower, "<!doctype html") ||
		strings.Contains(lower, "<html")
}

// zipIntegrity walks the whole archive and reads every byte, which surfaces
// truncated or corrupt downloads before we spend time extracting.
func zipIntegrity(path string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("cannot open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", f.Name, err)
		}
		if _, err := io.Copy(io.Discard, rc); err != nil {
			rc.Close()
			return fmt.Errorf("corrupt entry %s: %w", f.Name, err)
		}
		rc.Close()
	}
	return nil
}

// internalMD5 reads the hex digest that a Samsung .tar.md5 file carries in its
// first line, e.g. "1a2b3c...  AP_....tar.md5".
func internalMD5(tarMD5Path string) (string, error) {
	if info, err := os.Stat(tarMD5Path); err == nil && info.Size() > 4096 {
		digest, _, err := embeddedMD5(tarMD5Path)
		return digest, err
	}
	data, err := os.ReadFile(tarMD5Path)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty .tar.md5 file")
	}
	digest := strings.ToLower(fields[0])
	if len(digest) != md5.Size*2 {
		return "", fmt.Errorf("malformed md5 %q", fields[0])
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", fmt.Errorf("malformed md5 %q", fields[0])
	}
	return digest, nil
}

// actualMD5 hashes a .tar file on disk so it can be compared against the digest
// declared by its .tar.md5 sibling.
func actualMD5(tarPath string) (string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifyTarMD5 checks one .tar.md5 against its .tar sibling.
func verifyTarMD5(tarMD5Path string) error {
	if info, err := os.Stat(tarMD5Path); err == nil && info.Size() > 4096 {
		return verifyEmbeddedMD5(tarMD5Path)
	}
	return fmt.Errorf("invalid Samsung package: a text MD5 sidecar is not a flashable TAR")
}

// slotOf classifies a firmware member by its filename prefix.
func slotOf(fileName string) Slot {
	name := strings.ToUpper(fileName)
	switch {
	case strings.HasPrefix(name, "AP_"):
		return SlotAP
	case strings.HasPrefix(name, "BL_"):
		return SlotBL
	case strings.HasPrefix(name, "CP_"):
		return SlotCP
	case strings.HasPrefix(name, "CSC_"):
		return SlotCSC
	case strings.HasPrefix(name, "HOME_CSC_"):
		return SlotCSC
	}
	return ""
}

// ValidateFirmwareArchive runs the full battery of checks a Samsung package
// must pass before AutoRoot will consider flashing it. It is deliberately
// conservative: anything that cannot be positively proven is an error.
func ValidateFirmwareArchive(archivePath string, profile FirmwareProfile, extractDir string) *FirmwareValidation {
	v := &FirmwareValidation{Sizes: map[Slot]SlotFile{}}

	archivePath = strings.TrimSpace(archivePath)
	if archivePath == "" {
		v.fail("no firmware archive was provided")
		return v
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		v.fail("firmware archive not found: %s", archivePath)
		return v
	}
	if info.IsDir() {
		v.fail("firmware archive is a directory: %s", archivePath)
		return v
	}
	if looksLikeHTMLError(archivePath) {
		v.fail("firmware archive is an HTML page, not a Samsung ZIP (captured login/consent page or expired link)")
		return v
	}
	if info.Size() < minArchiveBytes {
		v.fail("firmware archive is suspiciously small (%d bytes) - this is usually a failed or partial download", info.Size())
		return v
	}

	// The archive name carries the model and binary revision. Reject a package
	// for another phone before spending time extracting 5.8 GB.
	wantCode := modelCodeOf(profile.Model)
	if wantCode != "" {
		archiveModel := samsungModelFromName(archivePath)
		if archiveModel == "" {
			v.warn("archive name has no build token; model and binary will be verified from its members")
		} else if archiveModel != wantCode {
			v.fail("model mismatch: archive is for %s but the phone is %s", archiveModel, wantCode)
		} else {
			v.ModelCode = archiveModel
		}

		// Anti-rollback: never accept a package older than the running binary.
		archiveBinary := binaryFromName(archivePath)
		if archiveBinary == "" {
			// Vendor archive names need not carry a build; members below are authoritative.
		} else {
			v.PkgBinary = archiveBinary
			deviceBinary := strings.ToUpper(strings.TrimSpace(profile.DeviceBinary))
			if deviceBinary != "" && deviceBinary != "N/A" {
				if _, ok := base36Value(deviceBinary[0]); !ok {
					v.fail("device reports an unreadable bootloader binary %q", deviceBinary)
				} else if compareRevisions(archiveBinary, deviceBinary) < 0 {
					v.fail("anti-rollback: package binary %s is lower than the device binary %s; Samsung will reject this downgrade",
						archiveBinary, deviceBinary)
				}
			}
		}
	}

	if len(v.Errors) > 0 {
		return v
	}

	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		v.fail("cannot create extraction folder: %v", err)
		return v
	}

	r, err := zip.OpenReader(archivePath)
	if err != nil {
		v.fail("cannot open zip: %v", err)
		return v
	}
	defer r.Close()

	for _, f := range r.File {
		updateJobProgress("Inspect ZIP member: "+f.Name, 0, 1)
		if f.FileInfo().IsDir() {
			continue
		}
		slot := slotOf(f.Name)
		if slot == "" {
			continue
		}
		dest := filepath.Join(extractDir, filepath.Base(f.Name))
		if !strings.HasSuffix(strings.ToUpper(f.Name), ".TAR.MD5") {
			if st, err := os.Stat(dest); err != nil || st.Size() != int64(f.UncompressedSize64) {
				if err := extractOne(f, dest); err != nil {
					v.fail("extract: %v", err)
					return v
				}
			}
			continue
		}
		// Reuse a previously extracted member when it is still byte-complete.
		// Re-extracting unconditionally would overwrite a tampered payload and
		// silently "fix" the very corruption this check exists to catch, and it
		// would re-read 5.8 GB on every validation pass.
		if st, err := os.Stat(dest); err == nil && st.Size() == int64(f.UncompressedSize64) {
			if slot == SlotCSC && strings.HasPrefix(upperBase(f.Name), "HOME_CSC_") && strings.HasPrefix(upperBase(v.Sizes[slot].FileName), "CSC_") {
				continue
			}
			v.Sizes[slot] = SlotFile{
				Slot:     slot,
				Path:     dest,
				FileName: filepath.Base(f.Name),
				Size:     st.Size(),
			}
			continue
		}
		if err := extractOne(f, dest); err != nil {
			v.fail("cannot extract %s: %v", f.Name, err)
			return v
		}
		st, err := os.Stat(dest)
		if err != nil {
			v.fail("extracted %s is missing: %v", f.Name, err)
			return v
		}
		if slot == SlotCSC && strings.HasPrefix(upperBase(f.Name), "HOME_CSC_") && strings.HasPrefix(upperBase(v.Sizes[slot].FileName), "CSC_") {
			continue
		}
		v.Sizes[slot] = SlotFile{
			Slot:     slot,
			Path:     dest,
			FileName: filepath.Base(f.Name),
			Size:     st.Size(),
		}
	}

	// A HOME_CSC package satisfies the CSC slot only for an already-rooted
	// phone; the first install needs a wipe-inducing CSC.
	_, sawHomeCSC := findHomeCSC(r)
	if sawHomeCSC {
		v.warn("package contains HOME_CSC: it preserves user data and can only be used for an update on an already-rooted phone")
	}

	for _, slot := range RequiredSlots {
		sf, ok := v.Sizes[slot]
		if !ok {
			v.fail("package is missing the %s partition", slot)
			continue
		}
		if !strings.HasSuffix(strings.ToUpper(sf.FileName), ".TAR.MD5") {
			v.fail("%s member %s is not a .tar.md5 package", slot, sf.FileName)
			continue
		}
		// Samsung .tar.md5 contains the TAR payload followed by its MD5 trailer.
		if sf.Size < 32 {
			v.fail("%s member %s is too small to contain an md5 digest (%d bytes)", slot, sf.FileName, sf.Size)
		}
	}

	// A renamed archive is a trivial attack: the AP/BL/CP/CSC members carry
	// their own model code, and they must all agree with the phone. Checking
	// only the outer filename would let a repackaged A055F package through.
	if wantCode != "" {
		for slot, sf := range v.Sizes {
			memberModel := samsungModelFromName(sf.FileName)
			switch {
			case memberModel == "":
				v.fail("%s member %s does not encode a recognisable model code", slot, sf.FileName)
			case memberModel != wantCode:
				v.fail("%s member %s is built for %s but the phone is %s", slot, sf.FileName, memberModel, wantCode)
			default:
				v.ModelCode = memberModel
				memberBinary := binaryFromName(sf.FileName)
				if len(profile.DeviceBinary) != 1 || memberBinary == "" {
					v.fail("unreadable device or member binary")
				} else if compareRevisions(memberBinary, profile.DeviceBinary) < 0 {
					v.fail("anti-rollback: %s binary %s is below %s", slot, memberBinary, profile.DeviceBinary)
				}
				if slot == SlotAP {
					v.PkgBinary = memberBinary
				}
			}
		}
	}

	// Every extracted .tar.md5 must match its .tar, which proves the download
	// was not truncated or corrupted in transit.
	for _, sf := range v.Sizes {
		var member *zip.File
		for _, f := range r.File {
			if filepath.Base(f.Name) == sf.FileName {
				member = f
				break
			}
		}
		if member == nil {
			v.fail("ZIP member missing: %s", sf.FileName)
			continue
		}
		if err := verifyEmbeddedMD5(sf.Path, member.CRC32); err != nil {
			v.fail("%s: %v", sf.FileName, err)
		}
	}

	// A single-CSC package must match the phone's sales code. Multi-CSC
	// packages embed the supported codes and are accepted with a warning.
	if profile.CSC != "" {
		if profile.CSCBuild != "" && strings.Contains(upperBase(v.Sizes[SlotCSC].FileName), strings.ToUpper(profile.CSCBuild)+"_") {
			v.CSCInPackage = strings.ToUpper(profile.CSC)
		}
		wantCSC := strings.ToUpper(strings.TrimSpace(profile.CSC))
		if v.ModelCode != "" {
			archiveName := strings.ToUpper(filepath.Base(archivePath))
			if strings.HasPrefix(archiveName, "CSC_") || strings.Contains(archiveName, "_CSC_") {
				if !strings.Contains(archiveName, wantCSC) {
					v.fail("CSC mismatch: package sales code does not contain %s", wantCSC)
				} else {
					v.CSCInPackage = wantCSC
				}
			} else if v.CSCInPackage == "" {
				v.warn("could not confirm %s is inside this package's CSC; verify manually before flashing", wantCSC)
			}
		}
	}

	v.OK = len(v.Errors) == 0
	return v
}

func findHomeCSC(r *zip.ReadCloser) (string, bool) {
	for _, f := range r.File {
		if strings.HasPrefix(strings.ToUpper(f.Name), "HOME_CSC_") {
			return f.Name, true
		}
	}
	return "", false
}

func extractOne(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.CreateTemp(filepath.Dir(dest), ".extract-*.part")
	if err != nil {
		return err
	}
	part := out.Name()
	defer os.Remove(part)

	_, err = io.Copy(out, &progressReader{reader: rc, total: int64(f.UncompressedSize64), detail: "Extract/CRC: " + f.Name})
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(part, dest)
}

// formatSize renders a byte count for user-facing messages.
func formatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
