package main

import (
	"strings"
	"testing"
)

func TestRecoveryPreflightCandidateLeavesOriginalFailedAndUnapprovedCandidate(t *testing.T) {
	s := &Session{Stage: StageFailed, Failure: "flash failed: fork/exec engine.exe: Access is denied.; no automatic retry", Approval: &FlashApproval{Granted: true, Fingerprint: "bound"}, Artifacts: map[string]string{"patchedAP": "new-patch.tar"}, Provenance: map[string]string{"validatorVersion": "embedded-md5-v1", "uiPatchState": "completed", "patchedAPSHA256": strings.Repeat("a", 64)}}
	candidate, err := launchRecoveryCandidate(s, "engine.exe")
	if err != nil {
		t.Fatal(err)
	}
	if !candidate.Reached(StagePatchedAP) || candidate.Approval != nil || candidate.Failure != "" {
		t.Fatal("candidate not suitable for preflight")
	}
	if s.Stage != StageFailed || s.Approval == nil || s.Failure == "" {
		t.Fatal("original failure mutated before validation/archive")
	}
	delete(s.Provenance, "validatorVersion")
	if _, err := launchRecoveryCandidate(s, "engine.exe"); err == nil {
		t.Fatal("missing validation accepted")
	}
	s.Provenance["flashLaunchState"] = "started"
	if _, err := launchRecoveryCandidate(s, "engine.exe"); err == nil {
		t.Fatal("started flash accepted")
	}
}

func TestLaunchRecoveryRejectsStartedAndUncertainFailures(t *testing.T) {
	engine := "engine.exe"
	s := &Session{Stage: StageFailed, Approval: &FlashApproval{Granted: true, Fingerprint: "bound"}, Provenance: map[string]string{}, Failure: "flash failed: fork/exec engine.exe: Access is denied.; no automatic retry"}
	if err := launchFailureRecoverable(s, engine); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"started", "unknown", "recovered-not-started"} {
		s.Provenance["flashLaunchState"] = state
		if err := launchFailureRecoverable(s, engine); err == nil {
			t.Fatalf("unsafe state accepted: %s", state)
		}
	}
	s.Provenance["flashLaunchState"] = "not-started"
	s.Provenance["flashEngine"] = engine
	if err := launchFailureRecoverable(s, engine); err != nil {
		t.Fatal(err)
	}
	s.Failure = "flash failed: exit status 1; no automatic retry"
	if err := launchFailureRecoverable(s, engine); err == nil {
		t.Fatal("exit failure accepted")
	}
}

func TestLaunchRecoveryRequiresConfirmationAndMatchingEvidence(t *testing.T) {
	if _, e := recoverEngineLaunch(map[string]interface{}{}); e == "" {
		t.Fatal("confirmation required")
	}
	if e := launchFailureRecoverable(nil, "engine.exe"); e == nil {
		t.Fatal("nil session accepted")
	}
	s := &Session{Stage: StageFailed, Approval: &FlashApproval{Granted: true, Fingerprint: "bound"}, Provenance: map[string]string{}, Failure: "flash failed: fork/exec other.exe: Access is denied.; no automatic retry"}
	if e := launchFailureRecoverable(s, "engine.exe"); e == nil {
		t.Fatal("wrong engine accepted")
	}
	s.Approval = nil
	if e := launchFailureRecoverable(s, "other.exe"); e == nil {
		t.Fatal("unbound files accepted")
	}
}
