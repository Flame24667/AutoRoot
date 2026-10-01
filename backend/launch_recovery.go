package main

import (
	"fmt"
	"strings"
)

// Accept only a proven failure of Cmd.Start, never an exit failure or an
// uncertain/started flash. The legacy match is the exact observed Go PathError.
func launchFailureRecoverable(s *Session, engine string) error {
	if s == nil || s.Stage != StageFailed || s.Approval == nil || !s.Approval.Granted || s.Approval.Fingerprint == "" {
		return fmt.Errorf("failed approved session required; no generic reset or retry")
	}
	state := s.Provenance["flashLaunchState"]
	if state == "not-started" && s.Provenance["flashEngine"] == engine && strings.HasPrefix(s.Failure, "flash failed: fork/exec "+engine+": ") {
		return nil
	}
	if state == "" && s.Failure == "flash failed: fork/exec "+engine+": Access is denied.; no automatic retry" {
		return nil
	}
	return fmt.Errorf("engine may have started; recovery blocked pending manual inspection")
}

// Read-only candidate for revalidation, not a persisted stage reset. The caller
// must still pass fresh preflight, PIT and old approval fingerprint checks and
// archive the original failure before writing the recovered session.
func launchRecoveryCandidate(s *Session, engine string) (*Session, error) {
	if err := launchFailureRecoverable(s, engine); err != nil {
		return nil, err
	}
	if s.Provenance["validatorVersion"] != "embedded-md5-v1" || s.Provenance["uiPatchState"] != "completed" || len(s.Provenance["patchedAPSHA256"]) != 64 || s.Artifact("patchedAP") == "" {
		return nil, fmt.Errorf("completed validated patch evidence required")
	}
	candidate := *s
	candidate.Stage = StagePatchedAP
	candidate.Approval = nil
	candidate.Failure = ""
	return &candidate, nil
}

func recoverEngineLaunch(payload interface{}) (interface{}, string) {
	p, _ := payload.(map[string]interface{})
	confirmed, _ := p["confirmRecovery"].(bool)
	serial, _ := p["serial"].(string)
	if !confirmed || serial == "" {
		return nil, "explicit recovery confirmation and serial required"
	}
	s := LoadSession()
	engine, err := resolveSamloader()
	if err != nil {
		return nil, err.Error()
	}
	if err = launchFailureRecoverable(s, engine); err != nil {
		return nil, err.Error()
	}
	if s.Serial != serial {
		return nil, "recovery serial does not match failed session"
	}
	diagnostic, diagError := diagnoseEngineLaunch(nil)
	if diagError != "" {
		return nil, diagError
	}
	d, ok := diagnostic.(map[string]interface{})
	if !ok || d["ok"] != true {
		return nil, fmt.Sprintf("engine launch diagnostic failed: %v", diagnostic)
	}
	check, _, err := runAdb("-s", serial, "shell", "command -v su; echo AUTOROOT_SU_CHECK:$?")
	if err != nil || strings.TrimSpace(check) != "AUTOROOT_SU_CHECK:1" {
		return nil, "stock/unrooted phone with no su executable required"
	}
	candidate, err := launchRecoveryCandidate(s, engine)
	if err != nil {
		return nil, err.Error()
	}
	report := RunPreflight(candidate)
	if !report.OK {
		return nil, "recovery preflight failed: " + strings.Join(report.Blockers, "; ")
	}
	binding := loadDownloadBinding(s)
	if binding == nil {
		return nil, "identity/PIT binding missing"
	}
	pitSum, err := FileSHA256(binding.PIT)
	if err != nil || pitSum != binding.PITSHA256 {
		return nil, "PIT evidence changed"
	}
	sum, err := flashFingerprint(s)
	if err != nil || sum != s.Approval.Fingerprint {
		return nil, "firmware/patch fingerprint changed since failed attempt"
	}
	archived, err := archiveSession(s)
	if err != nil {
		return nil, "cannot archive failure evidence: " + err.Error()
	}
	latest := LoadSession()
	if latest == nil || latest.UpdatedAt != s.UpdatedAt || latest.Failure != s.Failure || latest.Stage != StageFailed {
		return nil, "session changed during recovery; stop"
	}
	s.Provenance["recoveredFailure"] = s.Failure
	s.Provenance["recoveredFailureArchive"] = archived
	s.Provenance["flashLaunchState"] = "recovered-not-started"
	s.Stage = StagePatchedAP
	s.Approval = nil
	s.Failure = ""
	s.appendLog("Operator-confirmed pre-launch recovery; failure archived; approval revoked; no reboot or flash")
	if err = s.Save(); err != nil {
		return nil, err.Error()
	}
	return map[string]interface{}{"ok": true, "session": s, "archivedSession": archived, "message": "Pre-launch failure archived. Returned to patched-ap with NO approval. Refresh preflight and approve again before any flash. No phone data changed."}, ""
}
