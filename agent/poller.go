package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	const maxConsecutiveErrors = 5
	consecutiveErrors := 0

	log.Printf("[AGENT] started. server=%s machine=%s wait=%ds", p.cfg.ServerURL, p.cfg.MachineID, p.cfg.PollWait)

	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[AGENT] PANIC recovered: %v — restarting loop", r)
					consecutiveErrors = 0
				}
			}()

			task, err := p.poll()
			if err != nil {
				consecutiveErrors++
				log.Printf("[AGENT] poll error (%d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

				if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "403") {
					log.Printf("[AGENT] unauthorized — attempting re-registration")
					p.reRegister()
					consecutiveErrors = 0
				} else if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("[AGENT] too many errors, discovering new tunnel URL...")
					newURL := p.discoverURL()
					if newURL != "" && newURL != p.cfg.ServerURL {
						log.Printf("[AGENT] switching to new server: %s", newURL)
						p.cfg.ServerURL = newURL
						p.saveConfig()
					}
					consecutiveErrors = 0
				}

				time.Sleep(5 * time.Second)
				return
			}

			consecutiveErrors = 0

			if task == nil {
				return
			}

			if task.Action == "kill" {
				log.Printf("[AGENT] received kill command — shutting down")
				os.Exit(0)
			}

			log.Printf("[AGENT] executing task %s action=%s", task.TaskID, task.Action)
			p.executeAndReport(task)
		}()
	}
}

func (p *Poller) discoverURL() string {
	const gistURL = "https://gist.githubusercontent.com/egzakutacno/0c3de11a3381ae878b09626b306d04d1/raw/tunnel-url.txt"

	req, err := http.NewRequest("GET", gistURL, nil)
	if err != nil {
		return ""
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[AGENT] gist fetch error: %v", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(body))
	if strings.HasPrefix(url, "http") {
		return url
	}
	return ""
}

func (p *Poller) reRegister() {
	hostname, _ := os.Hostname()
	body := fmt.Sprintf(`{"name":"%s","api_key":"%s","hostname":"%s","metadata":"{}"}`,
		p.cfg.Name, p.cfg.APIKey, hostname)

	resp, err := p.client.Post(
		p.cfg.ServerURL+"/api/v1/agent/register",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		log.Printf("[AGENT] re-register failed: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[AGENT] re-registered successfully")
	} else {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("[AGENT] re-register returned %d: %s", resp.StatusCode, string(body))
	}
}

func (p *Poller) saveConfig() {
	data, err := json.MarshalIndent(p.cfg, "", "  ")
	if err != nil {
		return
	}

	exePath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(exePath), "agent.json")
	os.WriteFile(configPath, data, 0644)
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
