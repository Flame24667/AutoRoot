package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEngineDiagnosticCanOnlyRequestVersion(t *testing.T) {
	log, err := os.Create(filepath.Join(t.TempDir(), "diagnostic.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	cmd := engineVersionDiagnosticCommand(context.Background(), "engine.exe", log)
	if !reflect.DeepEqual(cmd.Args, []string{"engine.exe", "--version"}) {
		t.Fatalf("unsafe args: %v", cmd.Args)
	}
	if cmd.Stdout != log || cmd.Stderr != log || cmd.Stdin != nil || cmd.Dir != "" {
		t.Fatal("diagnostic must match flash stream/directory configuration")
	}
}
