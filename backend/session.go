package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Stage is a step in the root workflow. Stages are ordered and the workflow
// may only advance, never skip, so a resumed run cannot silently bypass a
// validation stage.
type Stage string

const (
	StageIdle          Stage = "idle"
	StageDeviceCheck   Stage = "device-check"
	StageAdbAuthorized  Stage = "adb-authorized"
	StageFirmwareFound Stage = "firmware-found"
	StageFirmwareValid Stage = "firmware-valid"
	StagePatchedAP     Stage = "patched-ap"
	StagePreflightOK   Stage = "preflight-ok"
	StageApprovedFlash Stage = "flash-approved"
	StageDownloadMode  Stage = "download-mode"
	StageFlashing      Stage = "flashing"
	StageWaitingBoot   Stage = "waiting-boot"
	StageVerifying     Stage = "verifying"
	StageRooted        Stage = "rooted"
	StageFailed        Stage = "failed"
)

// stageOrder gives each stage a rank so "did we already pass this?" is a
// numeric comparison rather than a string one. Stages that may repeat (a
// failed attempt that is retried) share a rank with their predecessor.
var stageOrder = map[Stage]int{
	StageIdle:          0,
	StageDeviceCheck:   1,
	StageAdbAuthorized: 2,
	StageFirmwareFound: 3,
	StageFirmwareValid: 4,
	StagePatchedAP:     5,
	StagePreflightOK:   6,
	StageApprovedFlash: 7,
	StageDownloadMode:  8,
	StageFlashing:      9,
	StageWaitingBoot:   10,
	StageVerifying:     11,
	StageRooted:        12,
	StageFailed:        0,
}

// Session is the persisted workflow state. It is written to disk on every
// transition so that closing the app, a USB drop, or a full reboot resumes
// from the recorded stage instead of from the beginning.
type Session struct {
	Serial       string            `json:"serial"`
	Model        string            `json:"model"`
	CSC          string            `json:"csc"`
	DeviceBinary string            `json:"deviceBinary"`
	Stage        Stage             `json:"stage"`
	UpdatedAt    string            `json:"updatedAt"`
	StartedAt    string            `json:"startedAt"`

	// Artifacts records where each produced file lives, so a resumed run can
	// reuse work already on disk.
	Artifacts map[string]string `json:"artifacts,omitempty"`

	// Provenance captures where firmware came from and how it was verified.
	Provenance map[string]string `json:"provenance,omitempty"`

	// Approval records the explicit consent gate. Flash is refused without it.
	Approval *FlashApproval `json:"approval,omitempty"`

	Failure string `json:"failure,omitempty"`
	Log     []string `json:"log,omitempty"`
}

// FlashApproval is the explicit consent captured before any destructive step.
// It exists because the flashing stage must never run unattended.
type FlashApproval struct {
	Granted     bool   `json:"granted"`
	GrantedBy   string `json:"grantedBy,omitempty"`
	GrantedAt   string `json:"grantedAt,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

var sessionMu sync.Mutex

// stateFile is the on-disk location of the persisted session.
func stateFile() string { return filepath.Join(writableStateDir(), "session.json") }

// LoadSession reads the persisted session, returning nil when none exists or
// the file is unreadable. A corrupt state file must never block startup; the
// caller simply restarts from idle.
func LoadSession() *Session {
	data, err := os.ReadFile(stateFile())
	if err != nil {
		return nil
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	if s.Artifacts == nil {
		s.Artifacts = map[string]string{}
	}
	if s.Provenance == nil {
		s.Provenance = map[string]string{}
	}
	return &s
}

// Save writes the session atomically so a crash mid-write cannot leave a
// truncated state file.
func (s *Session) Save() error {
	s.UpdatedAt = time.Now().Format(time.RFC3339)
	if s.StartedAt == "" {
		s.StartedAt = s.UpdatedAt
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := stateFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ClearSession removes the persisted state, used when a run is abandoned or
// completes successfully.
func ClearSession() error {
	err := os.Remove(stateFile())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// NewSession starts a workflow for a device, resetting any previous run for a
// different device.
func NewSession(serial, model, csc, deviceBinary string) *Session {
	return &Session{
		Serial:       serial,
		Model:        model,
		CSC:          csc,
		DeviceBinary: deviceBinary,
		Stage:        StageIdle,
		Artifacts:    map[string]string{},
		Provenance:   map[string]string{},
	}
}

// Advance moves the session to stage. A move that skips a stage is rejected
// outright rather than clamped: clamping would let a caller "arrive" at
// StageFlashing having never validated firmware, which is exactly the bypass
// the ordering exists to prevent.
func (s *Session) Advance(stage Stage) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	if s.Stage == StageFailed {
		return fmt.Errorf("session is in a failed state (%s); start a new run", s.Failure)
	}
	if stage == StageFailed {
		return nil
	}
	current, ok := stageOrder[s.Stage]
	if !ok {
		return fmt.Errorf("unknown current stage %q", s.Stage)
	}
	next, ok := stageOrder[stage]
	if !ok {
		return fmt.Errorf("unknown target stage %q", stage)
	}
	if next > current+1 {
		return fmt.Errorf("cannot advance from %s to %s: intermediate stages must be completed first", s.Stage, stage)
	}
	if next > current {
		s.Stage = stage
		s.appendLog(fmt.Sprintf("stage -> %s", stage))
		return s.Save()
	}
	// Re-entering an already-completed stage is a no-op, not an error, so a
	// resumed run can re-run an idempotent step.
	return nil
}

// Fail records a terminal error for this run.
func (s *Session) Fail(reason string) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	s.Stage = StageFailed
	s.Failure = reason
	s.appendLog("FAILED: " + reason)
	return s.Save()
}

// Reached reports whether the session has reached at least the given stage.
func (s *Session) Reached(stage Stage) bool {
	cur, ok := stageOrder[s.Stage]
	if !ok {
		return false
	}
	want, ok := stageOrder[stage]
	if !ok {
		return false
	}
	return cur >= want
}

// SetArtifact records the path of a produced file.
func (s *Session) SetArtifact(key, path string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if s.Artifacts == nil {
		s.Artifacts = map[string]string{}
	}
	s.Artifacts[key] = path
	s.appendLog(fmt.Sprintf("artifact %s -> %s", key, path))
	_ = s.Save()
}

// Artifact returns a previously produced path, or "" when absent.
func (s *Session) Artifact(key string) string {
	if s == nil || s.Artifacts == nil {
		return ""
	}
	return s.Artifacts[key]
}

// SetProvenance records where something came from and how it was verified.
func (s *Session) SetProvenance(key, value string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if s.Provenance == nil {
		s.Provenance = map[string]string{}
	}
	s.Provenance[key] = value
	_ = s.Save()
}

func (s *Session) appendLog(line string) {
	if s.Log == nil {
		s.Log = []string{}
	}
	stamp := time.Now().Format("15:04:05")
	s.Log = append(s.Log, stamp+" "+line)
	if len(s.Log) > 500 {
		s.Log = s.Log[len(s.Log)-500:]
	}
}

// RequireApproval returns an error unless the explicit consent gate has been
// granted for this run. Every destructive step calls it first.
func (s *Session) RequireApproval() error {
	if s.Approval == nil || !s.Approval.Granted {
		return fmt.Errorf("flashing requires explicit approval; none has been recorded for this run")
	}
	return nil
}

// GrantApproval records consent. It must only be called from an action that
// the user explicitly triggered.
func (s *Session) GrantApproval(by, fingerprint string) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	s.Approval = &FlashApproval{
		Granted:     true,
		GrantedBy:   by,
		GrantedAt:   time.Now().Format(time.RFC3339),
		Fingerprint: fingerprint,
	}
	s.appendLog("flash approved by " + by)
	return s.Save()
}

// MatchesDevice reports whether a persisted session belongs to the given
// device, so a session for another phone is never resumed by mistake.
func (s *Session) MatchesDevice(serial, model string) bool {
	if s == nil {
		return false
	}
	return s.Serial == serial && strings.EqualFold(s.Model, model)
}
