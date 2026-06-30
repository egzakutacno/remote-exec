package main

import "time"

type ExecResult struct {
	Output   string
	Error    string
	ExitCode int
}

type Runner interface {
	Execute(action string, payload string, timeout time.Duration) ExecResult
}
