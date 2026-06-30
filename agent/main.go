package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var activePoller *Poller

func main() {
	configPath := flag.String("config", "agent.json", "path to config file")
	registerFlag := flag.Bool("register", false, "register with server and exit")
	genConfig := flag.String("gen-config", "", "generate config template at path")
	consoleFlag := flag.Bool("console", false, "force console mode (no service)")
	flag.Parse()

	if *genConfig != "" {
		if err := WriteConfigTemplate(*genConfig); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("config template written to %s\n", *genConfig)
		return
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	setupLogging(cfg.LogFile)
	log.SetPrefix("")
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Printf("[AGENT] remote-exec-agent starting")
	log.Printf("[AGENT] server=%s machine=%s", cfg.ServerURL, cfg.MachineID)

	executor := NewWindowsExecutor()

	poller := NewPoller(cfg)
	poller.SetExecutor(executor)
	activePoller = poller

	if *registerFlag {
		registerAndExit(cfg)
		return
	}

	if *consoleFlag || runtime.GOOS != "windows" {
		log.Printf("[AGENT] running in console mode")
		poller.Run()
		return
	}

	if isServiceMode() {
		log.Printf("[AGENT] running as Windows service")
		serviceName := fmt.Sprintf("RemoteExecAgent-%s", cfg.MachineID)
		if cfg.Name != "" {
			serviceName = fmt.Sprintf("RemoteExecAgent-%s", cfg.Name)
		}
		err := runService(serviceName)
		if err != nil {
			log.Printf("[AGENT] service error: %v", err)
		}
		return
	}

	log.Printf("[AGENT] running in console mode")
	poller.Run()
}

func registerAndExit(cfg *Config) {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)

	if cfg.MachineID == "" || cfg.APIKey == "" {
		fmt.Fprintf(os.Stderr, "machine_id and api_key required in config\n")
		os.Exit(1)
	}

	fmt.Printf("Registration placeholder — edit %s and re-run without --register\n", filepath.Join(dir, "agent.json"))
}
