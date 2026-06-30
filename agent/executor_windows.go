package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type WindowsExecutor struct{}

func NewWindowsExecutor() *WindowsExecutor {
	return &WindowsExecutor{}
}

func (e *WindowsExecutor) Execute(action string, payload string, timeout time.Duration) ExecResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch action {
	case "ping":
		return ExecResult{Output: "pong", ExitCode: 0}
	case "run_powershell":
		return e.runPS(ctx, payload)
	case "run_cmd":
		return e.runCMD(ctx, payload)
	case "file_read":
		return e.fileRead(payload)
	case "file_write":
		return e.fileWrite(payload)
	case "file_delete":
		return e.fileDelete(payload)
	case "restart_service":
		return e.restartService(ctx, payload)
	case "install_package":
		return e.installPackage(ctx, payload)
	default:
		return ExecResult{
			Error:    fmt.Sprintf("unknown action: %s", action),
			ExitCode: 1,
		}
	}
}

func (e *WindowsExecutor) runPS(ctx context.Context, script string) ExecResult {
	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-Command", script,
	)

	return e.runCommand(cmd)
}

func (e *WindowsExecutor) runCMD(ctx context.Context, command string) ExecResult {
	cmd := exec.CommandContext(ctx, "cmd.exe", "/C", command)
	return e.runCommand(cmd)
}

func (e *WindowsExecutor) runCommand(cmd *exec.Cmd) ExecResult {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	errOutput := stderr.String()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			errOutput = err.Error() + "\n" + errOutput
		}
	}

	combined := output
	if errOutput != "" {
		if combined != "" {
			combined += "\n[STDERR]\n"
		}
		combined += errOutput
	}

	combined = strings.TrimSpace(combined)
	if len(combined) > 50000 {
		combined = combined[:50000] + "\n... [TRUNCATED]"
	}

	errStr := ""
	if exitCode != 0 {
		errStr = errOutput
	}

	return ExecResult{
		Output:   combined,
		Error:    errStr,
		ExitCode: exitCode,
	}
}

func (e *WindowsExecutor) fileRead(path string) ExecResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExecResult{Error: err.Error(), ExitCode: 1}
	}
	out := string(data)
	if len(out) > 50000 {
		out = out[:50000] + "\n... [TRUNCATED]"
	}
	return ExecResult{Output: out, ExitCode: 0}
}

func (e *WindowsExecutor) fileWrite(payload string) ExecResult {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return ExecResult{Error: "invalid payload: expected {\"path\":...,\"content\":...}", ExitCode: 1}
	}
	if err := os.WriteFile(req.Path, []byte(req.Content), 0644); err != nil {
		return ExecResult{Error: err.Error(), ExitCode: 1}
	}
	return ExecResult{Output: fmt.Sprintf("written %d bytes to %s", len(req.Content), req.Path), ExitCode: 0}
}

func (e *WindowsExecutor) fileDelete(path string) ExecResult {
	if err := os.Remove(path); err != nil {
		return ExecResult{Error: err.Error(), ExitCode: 1}
	}
	return ExecResult{Output: fmt.Sprintf("deleted %s", path), ExitCode: 0}
}

func (e *WindowsExecutor) restartService(ctx context.Context, serviceName string) ExecResult {
	out, err := exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile", "-Command",
		fmt.Sprintf("Restart-Service -Name '%s' -Force; if($?){'OK'}else{'FAIL'}", serviceName),
	).CombinedOutput()
	if err != nil {
		return ExecResult{Error: err.Error() + "\n" + string(out), ExitCode: 1}
	}
	return ExecResult{Output: strings.TrimSpace(string(out)), ExitCode: 0}
}

func (e *WindowsExecutor) installPackage(ctx context.Context, packageName string) ExecResult {
	out, err := exec.CommandContext(ctx, "cmd.exe", "/C",
		fmt.Sprintf("winget install --accept-source-agreements --accept-package-agreements %s 2>&1", packageName),
	).CombinedOutput()
	if err != nil {
		psOut, _ := exec.CommandContext(ctx, "powershell.exe",
			"-NoProfile", "-Command",
			fmt.Sprintf("winget install --accept-source-agreements --accept-package-agreements %s 2>&1", packageName),
		).CombinedOutput()
		return ExecResult{Output: string(psOut), Error: string(out), ExitCode: 1}
	}
	return ExecResult{Output: strings.TrimSpace(string(out)), ExitCode: 0}
}

func writeTempFile(name string, content string) error {
	return os.WriteFile(name, []byte(content), 0644)
}

func readFile(name string) (string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func logExec(action string, payload string, result ExecResult) {
	log.Printf("[EXEC] action=%s exit=%d output_len=%d",
		action, result.ExitCode, len(result.Output))
}
