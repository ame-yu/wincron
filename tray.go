package main

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type upcomingTrayJob struct {
	Name string
	Next time.Time
}

type trayController struct {
	app                           *application.App
	tray                          *application.SystemTray
	cronSvc                       *CronService
	settingsSvc                   *SettingsService
	showMainWindow                func(string)
	closeMainWindowForLightweight func()
	quit                          func()
}

func newTrayController(app *application.App, cronSvc *CronService, settingsSvc *SettingsService, showMainWindow func(string), closeMainWindowForLightweight func(), quit func()) *trayController {
	trayIcon, _ := fs.ReadFile(assets, "build/windows/icon.ico")
	tray := app.SystemTray.New()
	tray.SetLabel("WinCron")
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}

	controller := &trayController{
		app:                           app,
		tray:                          tray,
		cronSvc:                       cronSvc,
		settingsSvc:                   settingsSvc,
		showMainWindow:                showMainWindow,
		closeMainWindowForLightweight: closeMainWindowForLightweight,
		quit:                          quit,
	}

	controller.UpdateTooltip()
	controller.refreshMenu()

	tray.OnClick(func() {})
	tray.OnRightClick(func() {
		controller.refreshMenu()
		tray.OpenMenu()
	})
	tray.OnDoubleClick(func() {
		controller.showMainWindow("Home")
	})

	return controller
}

func (c *trayController) UpdateTooltip() {
	c.tray.SetTooltip(buildUpcomingTasksTooltip(c.cronSvc))
}

func (c *trayController) refreshMenu() {
	globalEnabled, err := c.cronSvc.GetGlobalEnabled()
	if err != nil {
		globalEnabled = true
	}

	lightweightMode := c.settingsSvc.getLightweightMode()

	trayMenu := application.NewMenu()
	trayMenu.Add("Open Home Page").OnClick(func(_ *application.Context) {
		c.showMainWindow("Home")
	})

	trayMenu.AddCheckbox("Lightweight Mode", lightweightMode).OnClick(func(_ *application.Context) {
		current := c.settingsSvc.getLightweightMode()
		next := !current
		_ = c.settingsSvc.SetLightweightMode(next)
		if next {
			c.closeMainWindowForLightweight()
		}
		c.refreshMenu()
	})

	toggleLabel := "Disable Wincron"
	if !globalEnabled {
		toggleLabel = "Enable WinCron"
	}
	trayMenu.Add(toggleLabel).OnClick(func(_ *application.Context) {
		current, err := c.cronSvc.GetGlobalEnabled()
		if err != nil {
			current = true
		}
		next := !current
		_ = c.cronSvc.SetGlobalEnabled(next)
		c.app.Event.Emit("globalEnabledChanged", next)
		c.refreshMenu()
		c.UpdateTooltip()
	})

	trayMenu.Add("Settings").OnClick(func(_ *application.Context) {
		c.showMainWindow("Settings")
	})

	trayMenu.Add("Quit").OnClick(func(_ *application.Context) {
		c.quit()
	})

	c.tray.SetMenu(trayMenu)
}

func sanitizeTrayText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func buildUpcomingTasksTooltip(cronSvc *CronService) string {
	globalEnabled, err := cronSvc.GetGlobalEnabled()
	if err == nil && !globalEnabled {
		return "All tasks are paused"
	}

	jobs, err := cronSvc.ListJobs()
	if err != nil {
		return "WinCron"
	}

	items := make([]upcomingTrayJob, 0, len(jobs))
	for _, job := range jobs {
		nextRun := strings.TrimSpace(job.NextRunAt)
		if nextRun == "" {
			continue
		}

		nextAt, err := time.Parse(time.RFC3339, nextRun)
		if err != nil {
			continue
		}

		name := sanitizeTrayText(job.Name)
		if name == "" {
			name = sanitizeTrayText(job.Command)
		}
		if name == "" {
			name = "Unnamed task"
		}

		items = append(items, upcomingTrayJob{
			Name: name,
			Next: nextAt.Local(),
		})
	}

	if len(items) == 0 {
		return "No upcoming tasks"
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Next.Before(items[j].Next)
	})

	limit := 3
	if len(items) < limit {
		limit = len(items)
	}

	lines := make([]string, 0, limit)
	for _, item := range items[:limit] {
		lines = append(lines, fmt.Sprintf("%s (%s)", truncateRunes(item.Name, 22), item.Next.Format("01-02 15:04")))
	}
	return strings.Join(lines, "\n")
}
