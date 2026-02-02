package main

import (
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist build/windows/icon.ico
var assets embed.FS

func main() {
	args := os.Args[1:]
	consoleEnabled := false
	filteredArgs := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--console" {
			consoleEnabled = true
			continue
		}
		filteredArgs = append(filteredArgs, a)
	}
	if consoleEnabled {
		enableConsole()
	}

	isServiceCmd := len(filteredArgs) > 0 && filteredArgs[0] == "service"
	isServiceRun := isServiceCmd && len(filteredArgs) > 1 && filteredArgs[1] == "run"
	if !isServiceCmd || isServiceRun {
		release, alreadyRunning, err := acquireSingleInstanceLock()
		if err != nil {
			log.Fatal(err)
		}
		if alreadyRunning {
			if consoleEnabled {
				fmt.Println("wincron is already running")
			}
			return
		}
		defer release()
	}

	handled, err := handleServiceCommand(filteredArgs)
	if handled {
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	cronSvc := NewCronService()
	settingsSvc := NewSettingsService()
	configSvc := NewConfigService(cronSvc, settingsSvc)
	executedCh := make(chan JobLogEntry, 16)
	var quitting atomic.Bool

	app := application.New(application.Options{
		Name:        "wincron",
		Description: "A demo of using raw HTML & CSS",
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
		cronSvc.scheduler.Stop()
	})

	var mainWindowMu sync.Mutex
	var mainWindow *application.WebviewWindow
	var lightweightClosing atomic.Bool

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	createMainWindow := func() *application.WebviewWindow {
		windowW, windowH := settingsSvc.getWindowSize()
		windowOptions := application.WebviewWindowOptions{
			Title: "Window 1",
			BackgroundColour: application.NewRGB(246, 247, 251),
			URL:              "/",
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

			// Internal close for enabling lightweight mode should not be affected by closeBehavior.
			if lightweightClosing.Load() {
				mainWindowMu.Lock()
				if mainWindow == w {
					mainWindow = nil
				}
				mainWindowMu.Unlock()
				return
			}

			if settingsSvc.getCloseBehavior() == CloseBehaviorExit {
				quitting.Store(true)
				app.Quit()
				return
			}

			// CloseBehaviorTray: either hide window, or destroy webview when lightweight mode is enabled.
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

	ensureMainWindow := func() *application.WebviewWindow {
		mainWindowMu.Lock()
		defer mainWindowMu.Unlock()
		if mainWindow == nil {
			mainWindow = createMainWindow()
		}
		return mainWindow
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
		mainWindow = createMainWindow()
	}

	trayIcon, _ := fs.ReadFile(assets, "build/windows/icon.ico")
	tray := app.SystemTray.New()
	tray.SetLabel("WinCron")
	tray.SetTooltip("WinCron")
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}

	var setTrayMenu func()
	setTrayMenu = func() {
		globalEnabled, err := cronSvc.GetGlobalEnabled()
		if err != nil {
			globalEnabled = true
		}

		lightweightMode := settingsSvc.getLightweightMode()

		trayMenu := application.NewMenu()
		trayMenu.Add("Open Home Page").OnClick(func(_ *application.Context) {
			w := ensureMainWindow()
			w.Show()
			w.Focus()
			app.Event.Emit("navigate", "main")
		})

		trayMenu.AddCheckbox("Lightweight Mode", lightweightMode).OnClick(func(_ *application.Context) {
			current := settingsSvc.getLightweightMode()
			next := !current
			_ = settingsSvc.SetLightweightMode(next)
			if next {
				closeMainWindowForLightweight()
			}
			setTrayMenu()
		})

		toggleLabel := "Disable Wincron"
		if !globalEnabled {
			toggleLabel = "Enable WinCron"
		}
		trayMenu.Add(toggleLabel).OnClick(func(_ *application.Context) {
			current, err := cronSvc.GetGlobalEnabled()
			if err != nil {
				current = true
			}
			next := !current
			_ = cronSvc.SetGlobalEnabled(next)
			app.Event.Emit("globalEnabledChanged", next)
			setTrayMenu()
		})

		trayMenu.Add("Settings").OnClick(func(_ *application.Context) {
			w := ensureMainWindow()
			w.Show()
			w.Focus()
			app.Event.Emit("navigate", "Settings")
		})

		trayMenu.Add("Quit").OnClick(func(_ *application.Context) {
			quitting.Store(true)
			app.Quit()
		})

		tray.SetMenu(trayMenu)
	}
	setTrayMenu()
	tray.OnClick(func() {})
	tray.OnRightClick(func() {
		setTrayMenu()
		tray.OpenMenu()
	})
	tray.OnDoubleClick(func() {
		w := ensureMainWindow()
		w.Show()
		w.Focus()
		app.Event.Emit("navigate", "main")
	})

	cronSvc.setOnExecuted(func(entry JobLogEntry) {
		select {
		case executedCh <- entry:
		default:
		}
	})

	go func() {
		for entry := range executedCh {
			app.Event.Emit("jobExecuted", entry)

			status := "OK"
			if entry.ExitCode != 0 {
				status = fmt.Sprintf("ERR (exit=%d)", entry.ExitCode)
			}

			finishedHHMM := ""
			if entry.FinishedAt != "" {
				if t, err := time.Parse(time.RFC3339, entry.FinishedAt); err == nil {
					finishedHHMM = t.Local().Format("15:04")
				}
			}
			if finishedHHMM != "" {
				tray.SetTooltip(fmt.Sprintf("WinCron\n%s: %s (%s)", entry.JobName, status, finishedHHMM))
			} else {
				tray.SetTooltip(fmt.Sprintf("WinCron\n%s: %s", entry.JobName, status))
			}
		}
	}()

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	// Run the application. This blocks until the application has been exited.
	err = app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
