package main

import (
	"bytes"
	"context"
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
