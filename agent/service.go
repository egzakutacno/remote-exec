package main

import (
	"log"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
)

type agentService struct {
	poller *Poller
}

func (s *agentService) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	status <- svc.Status{State: svc.StartPending}

	go s.poller.Run()

	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			status <- c.CurrentStatus
			time.Sleep(100 * time.Millisecond)
			status <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			log.Printf("[AGENT] service stopping")
			status <- svc.Status{State: svc.StopPending}
			return false, 0
		default:
			log.Printf("[AGENT] unexpected control: %v", c.Cmd)
		}
	}

	return false, 0
}

func runService(name string) error {
	return svc.Run(name, &agentService{poller: activePoller})
}

func isServiceMode() bool {
	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Printf("failed check service mode: %v", err)
		return false
	}
	return inService
}

func setupLogging(logFile string) {
	if logFile == "" {
		return
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("cannot open log file %s: %v — using stderr only", logFile, err)
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}
