package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestRootIdentityNeedsExactUID(t *testing.T) {
	for _, out := range []string{"uid=0(root) gid=0(root)", "\nuid=0(root)\n"} {
		if !rootIdentity(out) {
			t.Fatal(out)
		}
	}
	for _, out := range []string{"uid=00(root)", "uid=2000(shell)", "xuid=0(root)", "uid=0(root)bad", ""} {
		if rootIdentity(out) {
			t.Fatal(out)
		}
	}
}

func TestNewRunGates(t *testing.T) {
	d := map[string]interface{}{"serial": "phone", "model": "SM-A065F", "rooted": false, "bootloaderLocked": false}
	s := NewSession("phone", "SM-A065F", "XID", "4")
	s.Stage = StageRooted // historical state can be reset only after a stock restore
	if err := validateNewRun(s, d, true, "phone", true); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []Stage{StageFlashing, StageDownloadMode} {
		s.Stage = stage
		if validateNewRun(s, d, true, "phone", true) == nil {
			t.Fatal("unresolved flash reset")
		}
	}
	s.Stage = StageRooted
	if validateNewRun(s, d, false, "phone", true) == nil || validateNewRun(s, d, true, "other", true) == nil || validateNewRun(s, d, true, "phone", false) == nil {
		t.Fatal("confirmation or su gate bypass")
	}
	for _, key := range []string{"rooted", "bootloaderLocked"} {
		d[key] = true
		if validateNewRun(s, d, true, "phone", true) == nil {
			t.Fatal(key)
		}
		d[key] = false
	}
	d["model"] = "SM-A055F"
	if validateNewRun(s, d, true, "phone", true) == nil {
		t.Fatal("wrong model")
	}
}

func TestArchivePreservesEvidenceWithoutReset(t *testing.T) {
	withTempState(t)
	s := NewSession("phone", "SM-A065F", "XID", "4")
	s.Stage = StageRooted
	s.Provenance["rootProof"] = "uid=0(root)"
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	path, err := archiveSession(s)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved Session
	if json.Unmarshal(data, &saved) != nil || saved.Stage != StageRooted || saved.Provenance["rootProof"] != "uid=0(root)" {
		t.Fatal("archive lost evidence")
	}
	if LoadSession().Stage != StageRooted {
		t.Fatal("archiving reset current session")
	}
}
