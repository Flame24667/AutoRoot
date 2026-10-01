package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Fixed --version only: no caller-controlled arguments, device calls or session changes.
// Match flash's working directory, nil stdin and shared file stdout/stderr.
func engineVersionDiagnosticCommand(ctx context.Context, engine string, log *os.File) *exec.Cmd {
	cmd := exec.CommandContext(ctx, engine, "--version")
	cmd.Stdout = log
	cmd.Stderr = log
	return cmd
}

func diagnoseEngineLaunch(_ interface{}) (interface{}, string) {
	engine, err := resolveSamloader()
	if err != nil {
		return nil, err.Error()
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err.Error()
	}
	path := filepath.Join(logDir(), fmt.Sprintf("engine-version-%d.log", time.Now().UnixNano()))
	log, err := os.Create(path)
	if err != nil {
		return nil, "cannot create diagnostic log: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := engineVersionDiagnosticCommand(ctx, engine, log)
	result := map[string]interface{}{
		"operation": "engineLaunchDiagnostic", "command": "--version", "engine": engine,
		"cwd": cwd, "logPath": path, "phoneTouched": false, "sessionChanged": false,
		"started": false, "ok": false,
	}
	err = cmd.Start()
	if err == nil {
		result["started"] = true
		result["pid"] = cmd.Process.Pid
		err = cmd.Wait()
		result["exitCode"] = cmd.ProcessState.ExitCode()
	}
	closeErr := log.Close()
	if err != nil {
		result["error"] = err.Error()
		result["errorType"] = fmt.Sprintf("%T", err)
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			result["systemError"] = pathErr.Err.Error()
		}
	}
	if closeErr != nil {
		result["logError"] = closeErr.Error()
	}
	output, readErr := os.ReadFile(path)
	if readErr != nil {
		result["logError"] = readErr.Error()
	} else {
		result["output"] = strings.TrimSpace(string(output))
		result["ok"] = err == nil && closeErr == nil && strings.TrimSpace(string(output)) == "samloader "+samloaderVersion
	}
	return result, ""
}
