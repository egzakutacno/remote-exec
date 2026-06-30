package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Task struct {
	TaskID  string `json:"task_id"`
	Action  string `json:"action"`
	Payload string `json:"payload"`
	Timeout int    `json:"timeout"`
}

type NextTaskResponse struct {
	Task    *Task  `json:"task"`
	Message string `json:"message"`
}

type ResultRequest struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Output   string `json:"output"`
	Error    string `json:"error"`
	ExitCode *int   `json:"exit_code"`
}

type Poller struct {
	cfg    *Config
	client *http.Client
	exec   Runner
}

func NewPoller(cfg *Config) *Poller {
	return &Poller{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.MaxTimeout) * time.Second,
		},
	}
}

func (p *Poller) SetExecutor(r Runner) {
	p.exec = r
}

func (p *Poller) Run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[AGENT] PANIC: %v", r)
			time.Sleep(2 * time.Second)
		}
	}()

	log.Printf("[AGENT] started. server=%s machine=%s wait=%ds", p.cfg.ServerURL, p.cfg.MachineID, p.cfg.PollWait)

	for {
		task, err := p.poll()
		if err != nil {
			log.Printf("[AGENT] poll error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if task == nil {
			continue
		}

		if task.Action == "kill" {
			log.Printf("[AGENT] received kill command — shutting down")
			return
		}

		log.Printf("[AGENT] executing task %s action=%s", task.TaskID, task.Action)
		p.executeAndReport(task)
	}
}

func (p *Poller) poll() (*Task, error) {
	url := fmt.Sprintf("%s/api/v1/agent/next-task?wait=%d",
		p.cfg.ServerURL, p.cfg.PollWait)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Machine-Id", p.cfg.MachineID)
	req.Header.Set("X-API-Key", p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var ntr NextTaskResponse
	if err := json.Unmarshal(body, &ntr); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return ntr.Task, nil
}

func (p *Poller) executeAndReport(task *Task) {
	timeout := time.Duration(task.Timeout) * time.Second
	if task.Timeout <= 0 {
		timeout = 30 * time.Second
	}

	result := p.exec.Execute(task.Action, task.Payload, timeout)

	p.report(task.TaskID, result)
}

func (p *Poller) report(taskID string, result ExecResult) {
	status := "success"
	if result.Error != "" {
		status = "error"
	}

	rr := ResultRequest{
		TaskID:   taskID,
		Status:   status,
		Output:   result.Output,
		Error:    result.Error,
		ExitCode: &result.ExitCode,
	}

	data, _ := json.Marshal(rr)
	url := fmt.Sprintf("%s/api/v1/agent/result", p.cfg.ServerURL)

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		log.Printf("[AGENT] report error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Machine-Id", p.cfg.MachineID)
	req.Header.Set("X-API-Key", p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[AGENT] report error: %v", err)
		return
	}
	resp.Body.Close()

	log.Printf("[AGENT] task %s reported: %s (exit=%d)", taskID, status, result.ExitCode)
}
