package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const windowsServiceName = "wincron"

func handleServiceCommand(args []string) (handled bool, err error) {
	if len(args) == 0 {
		return false, nil
	}
	if args[0] != "service" {
		return false, nil
	}
	if len(args) < 2 {
		return true, errors.New("usage: service <install|uninstall|start|stop|run>")
	}

	sub := args[1]
	switch sub {
	case "install":
		return true, installService()
	case "uninstall":
		return true, uninstallService()
	case "start":
		return true, startService()
	case "stop":
		return true, stopService()
	case "run":
		return true, runService()
	default:
		return true, fmt.Errorf("unknown service subcommand: %s", sub)
	}
}

func installService() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	if s, err := m.OpenService(windowsServiceName); err == nil {
		s.Close()
		return errors.New("service already exists")
	}

	config := mgr.Config{
		DisplayName:      "WinCron",
		StartType:        mgr.StartAutomatic,
		DelayedAutoStart: true,
	}

	s, err := m.CreateService(windowsServiceName, exePath, config, "service", "run")
	if err != nil {
		return err
	}
	defer s.Close()
	return nil
}

func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Delete()
}

func startService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Start()
}

func stopService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return err
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		return err
	}

	timeout := time.Now().Add(15 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(timeout) {
			return errors.New("timeout waiting for service to stop")
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return err
		}
	}
	return nil
}

func runService() error {
	return svc.Run(windowsServiceName, &windowsServiceHandler{})
}

type windowsServiceHandler struct{}

func (h *windowsServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending}

	cronSvc := NewCronService()

	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				cronSvc.scheduler.Stop()
				s <- svc.Status{State: svc.StopPending}
				s <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		}
	}
}
