package main

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wincron/internal/ipc"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist build/windows/icon.ico
var assets embed.FS

func boolPtr(v bool) *bool {
	return &v
}

func main() {
	args := os.Args[1:]

	release, alreadyRunning, err := acquireSingleInstanceLock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if alreadyRunning {
		if len(args) == 0 {
			deadline := time.Now().Add(1500 * time.Millisecond)
			for {
				_, err := ipc.SendRequest(ipc.Request{Cmd: "open"})
				if err == nil {
					break
				}
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return
	}
	defer release()

	cronSvc := NewCronService()
	settingsSvc := NewSettingsService()
	configSvc := NewConfigService(cronSvc, settingsSvc)
	var quitting atomic.Bool

	currentBootTime := GetSystemBootTime()
	if settingsSvc.getAutoStart() && settingsSvc.IsSystemRebooted() {
		go cronSvc.RunStartupJobs()
		_ = settingsSvc.SetLastSystemBootTime(currentBootTime)
	}

	app := application.New(application.Options{
		Name:        "WinCron",
		Description: "A cron job scheduler for Windows",
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(cronSvc),
			application.NewService(settingsSvc),
			application.NewService(configSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.OnShutdown(func() {
		cronSvc.stopHotkeys()
		cronSvc.scheduler.Stop()
	})

	var mainWindowMu sync.Mutex
	var mainWindow *application.WebviewWindow
	var lightweightClosing atomic.Bool

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	createMainWindow := func(target string) *application.WebviewWindow {
		url := "/"
		if target == "Settings" {
			url = "/#/settings"
		}
		windowW, windowH := settingsSvc.getWindowSize()
		windowOptions := application.WebviewWindowOptions{
			Title:            "WinCron",
			BackgroundColour: application.NewRGB(246, 247, 251),
			URL:              url,
		}
		if windowW > 0 && windowH > 0 {
			windowOptions.Width = windowW
			windowOptions.Height = windowH
		}
		w := app.Window.NewWithOptions(windowOptions)

		var resizeMu sync.Mutex
		var resizeTimer *time.Timer
		var pendingW, pendingH int
		w.RegisterHook(events.Common.WindowDidResize, func(_ *application.WindowEvent) {
			cw, ch := w.Size()
			resizeMu.Lock()
			pendingW, pendingH = cw, ch
			if resizeTimer != nil {
				_ = resizeTimer.Stop()
			}
			resizeTimer = time.AfterFunc(600*time.Millisecond, func() {
				resizeMu.Lock()
				sw, sh := pendingW, pendingH
				resizeMu.Unlock()
				_ = settingsSvc.setWindowSize(sw, sh)
			})
			resizeMu.Unlock()
		})

		w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			if quitting.Load() {
				return
			}

			// Internal close for enabling lightweight mode should not be affected by runInTray.
			if lightweightClosing.Load() {
				mainWindowMu.Lock()
				if mainWindow == w {
					mainWindow = nil
				}
				mainWindowMu.Unlock()
				return
			}

			if !settingsSvc.getRunInTray() {
				quitting.Store(true)
				app.Quit()
				return
			}

			// RunInTray: either hide window, or destroy webview when lightweight mode is enabled.
			if settingsSvc.getLightweightMode() {
				mainWindowMu.Lock()
				if mainWindow == w {
					mainWindow = nil
				}
				mainWindowMu.Unlock()
				return
			}

			w.Hide()
			e.Cancel()
		})

		return w
	}

	showMainWindow := func(target string) {
		mainWindowMu.Lock()
		created := false
		if mainWindow == nil {
			mainWindow = createMainWindow(target)
			created = true
		}
		w := mainWindow
		mainWindowMu.Unlock()
		w.Show()
		w.Focus()
		if !created {
			app.Event.Emit("navigate", target)
		}
	}

	ipcStop, ipcErr := ipc.StartServer(ipc.ControlPipeUserPath(), false, func(req ipc.Request) ipc.Response {
		target := strings.TrimSpace(req.Target)

		matchJobsByNameOrFolder := func(t string) ([]Job, error) {
			jobs, err := cronSvc.ListJobs()
			if err != nil {
				return nil, err
			}
			matched := make([]Job, 0)
			for _, j := range jobs {
				if strings.EqualFold(strings.TrimSpace(j.Name), t) || strings.EqualFold(strings.TrimSpace(j.Folder), t) {
					matched = append(matched, j)
				}
			}
			return matched, nil
		}

		matchJobsByName := func(t string) ([]Job, error) {
			jobs, err := cronSvc.ListJobs()
			if err != nil {
				return nil, err
			}
			matched := make([]Job, 0)
			for _, j := range jobs {
				if strings.EqualFold(strings.TrimSpace(j.Name), t) {
					matched = append(matched, j)
				}
			}
			return matched, nil
		}

		setEnabledByTarget := func(t string, enabled bool) ipc.Response {
			matched, err := matchJobsByNameOrFolder(t)
			if err != nil {
				return ipc.Response{Ok: false, Error: err.Error()}
			}
			if len(matched) == 0 {
				return ipc.Response{Ok: false, Error: fmt.Sprintf("no matching jobs: %s", t)}
			}
			success := 0
			failed := 0
			for _, j := range matched {
				if _, err := cronSvc.SetJobEnabled(j.ID, enabled); err != nil {
					failed++
					continue
				}
				success++
			}
			if success == 0 {
				return ipc.Response{Ok: false, Error: "failed to update matched jobs"}
			}
			verb := "disabled"
			if enabled {
				verb = "enabled"
			}
			msg := fmt.Sprintf("%s %d job(s)", verb, success)
			if failed > 0 {
				msg = fmt.Sprintf("%s, %d failed", msg, failed)
			}
			return ipc.Response{Ok: true, Message: msg}
		}

		switch req.Cmd {
		case "disable":
			if target == "" {
				if err := cronSvc.SetGlobalEnabled(false); err != nil {
					return ipc.Response{Ok: false, Error: err.Error()}
				}
				app.Event.Emit("globalEnabledChanged", false)
				return ipc.Response{Ok: true, Message: "WinCron disabled", GlobalEnabled: boolPtr(false)}
			}
			return setEnabledByTarget(target, false)
		case "enable":
			if target == "" {
				if err := cronSvc.SetGlobalEnabled(true); err != nil {
					return ipc.Response{Ok: false, Error: err.Error()}
				}
				app.Event.Emit("globalEnabledChanged", true)
				return ipc.Response{Ok: true, Message: "WinCron enabled", GlobalEnabled: boolPtr(true)}
			}
			return setEnabledByTarget(target, true)
		case "status":
			v, err := cronSvc.GetGlobalEnabled()
			if err != nil {
				return ipc.Response{Ok: false, Error: err.Error()}
			}
			return ipc.Response{Ok: true, GlobalEnabled: boolPtr(v)}
		case "run":
			if target == "" {
				return ipc.Response{Ok: false, Error: "job name is required"}
			}
			matched, err := matchJobsByName(target)
			if err != nil {
				return ipc.Response{Ok: false, Error: err.Error()}
			}
			if len(matched) == 0 {
				return ipc.Response{Ok: false, Error: fmt.Sprintf("no matching jobs: %s", target)}
			}
			for _, j := range matched {
				id := j.ID
				go func() {
					_, _ = cronSvc.runNow(id, logTriggerSourceIPC)
				}()
			}
			return ipc.Response{Ok: true, Message: fmt.Sprintf("started %d job(s)", len(matched))}
		case "import":
			if strings.TrimSpace(req.Payload) == "" {
				return ipc.Response{Ok: false, Error: "import payload is required"}
			}
			if err := configSvc.ImportYAML(req.Payload, req.ConflictStrategy); err != nil {
				return ipc.Response{Ok: false, Error: err.Error()}
			}
			return ipc.Response{Ok: true, Message: "imported"}
		case "open":
			showMainWindow("Home")
			return ipc.Response{Ok: true, Message: "ok"}
		case "quit":
			quitting.Store(true)
			app.Quit()
			return ipc.Response{Ok: true, Message: "ok"}
		default:
			return ipc.Response{Ok: false, Error: "unknown command"}
		}
	})
	if ipcErr == nil {
		app.OnShutdown(func() {
			ipcStop()
		})
	}

	closeMainWindowForLightweight := func() {
		mainWindowMu.Lock()
		w := mainWindow
		mainWindow = nil
		mainWindowMu.Unlock()
		if w == nil {
			return
		}
		lightweightClosing.Store(true)
		w.Close()
		lightweightClosing.Store(false)
	}

	if !settingsSvc.getSilentStart() {
		mainWindow = createMainWindow("Home")
	}

	trayController := newTrayController(app, cronSvc, settingsSvc, showMainWindow, closeMainWindowForLightweight, func() {
		quitting.Store(true)
		app.Quit()
	})
	emitJobEvent := func(name string) func(JobLogEntry) {
		return func(entry JobLogEntry) {
			app.Event.Emit(name, entry)
		}
	}
	cronSvc.setOnJobsChanged(trayController.UpdateTooltip)

	cronSvc.setOnStarted(emitJobEvent("jobStarted"))
	cronSvc.setOnExecuted(emitJobEvent("jobExecuted"))

	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for now := range ticker.C {
			app.Event.Emit("time", now.Format(time.RFC1123))
		}
	}()

	// Run the application. This blocks until the application has been exited.
	err = app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
