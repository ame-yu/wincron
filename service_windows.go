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

func withWindowsService(fn func(*mgr.Service) error) error {
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

	return fn(s)
}

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
	return withWindowsService(func(s *mgr.Service) error {
		return s.Delete()
	})
}

func startService() error {
	return withWindowsService(func(s *mgr.Service) error {
		return s.Start()
	})
}

func stopService() error {
	return withWindowsService(func(s *mgr.Service) error {
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
	})
}

func runService() error {
	return svc.Run(windowsServiceName, &windowsServiceHandler{})
}

type windowsServiceHandler struct{}

func (h *windowsServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending}

	cronSvc := NewCronService()
	quitCh := make(chan struct{}, 1)

	ipcStop, ipcErr := startIPCServer(wincronControlPipeServicePath, false, func(req ipcRequest) ipcResponse {
		switch req.Cmd {
		case "disable":
			if err := cronSvc.SetGlobalEnabled(false); err != nil {
				return ipcResponse{Ok: false, Error: err.Error()}
			}
			return ipcResponse{Ok: true, Message: "\u5df2\u7981\u7528 WinCron", GlobalEnabled: boolPtr(false)}
		case "enable":
			if err := cronSvc.SetGlobalEnabled(true); err != nil {
				return ipcResponse{Ok: false, Error: err.Error()}
			}
			return ipcResponse{Ok: true, Message: "\u5df2\u542f\u7528 WinCron", GlobalEnabled: boolPtr(true)}
		case "status":
			v, err := cronSvc.GetGlobalEnabled()
			if err != nil {
				return ipcResponse{Ok: false, Error: err.Error()}
			}
			return ipcResponse{Ok: true, GlobalEnabled: boolPtr(v)}
		case "quit":
			select {
			case quitCh <- struct{}{}:
			default:
			}
			return ipcResponse{Ok: true, Message: "ok"}
		case "open":
			return ipcResponse{Ok: false, Error: "open not supported in service mode"}
		default:
			return ipcResponse{Ok: false, Error: "unknown command"}
		}
	})
	if ipcErr != nil {
		ipcStop = func() {}
	}

	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case <-quitCh:
			ipcStop()
			cronSvc.scheduler.Stop()
			s <- svc.Status{State: svc.StopPending}
			s <- svc.Status{State: svc.Stopped}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				ipcStop()
				cronSvc.scheduler.Stop()
				s <- svc.Status{State: svc.StopPending}
				s <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		}
	}
}
