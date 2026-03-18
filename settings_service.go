package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

type AppSettings struct {
	LightweightMode bool `json:"lightweightMode,omitempty" yaml:"lightweightMode,omitempty"`
	SilentStart     bool `json:"silentStart,omitempty" yaml:"silentStart,omitempty"`
	AutoStart       bool `json:"autoStart,omitempty" yaml:"autoStart,omitempty"`
	RunInTray       bool `json:"runInTray" yaml:"runInTray"`
}

func defaultAppSettings() AppSettings {
	return AppSettings{
		RunInTray:       true,
		LightweightMode: true,
	}
}

type LocalSettings struct {
	WindowWidth        int    `json:"windowWidth,omitempty"`
	WindowHeight       int    `json:"windowHeight,omitempty"`
	LastSystemBootTime string `json:"lastSystemBootTime,omitempty"`
}

type settingsStoreData struct {
	AppSettings
	Local LocalSettings `json:"local"`
}

func defaultSettingsStoreData() settingsStoreData {
	return settingsStoreData{
		AppSettings: defaultAppSettings(),
	}
}

func normalizeWindowSize(width int, height int) (int, int) {
	if width < 200 || height < 200 {
		return 0, 0
	}
	return width, height
}

func (d *settingsStoreData) normalize() {
	d.Local.WindowWidth, d.Local.WindowHeight = normalizeWindowSize(d.Local.WindowWidth, d.Local.WindowHeight)
}

type settingsStore struct {
	path string
}

func newSettingsStore(path string) *settingsStore {
	return &settingsStore{path: path}
}

func (s *settingsStore) load() (settingsStoreData, error) {
	data := defaultSettingsStoreData()
	if err := readJSONOrDefault(s.path, &data, func() {
		data = defaultSettingsStoreData()
	}); err != nil {
		return settingsStoreData{}, err
	}
	data.normalize()
	return data, nil
}

func (s *settingsStore) save(data settingsStoreData) error {
	return writeJSONAtomic(s.path, data)
}

type SettingsService struct {
	mu    sync.RWMutex
	store *settingsStore
	data  settingsStoreData
}

func NewSettingsService() *SettingsService {
	baseDir := defaultDataDir()
	store := newSettingsStore(filepath.Join(baseDir, "settings.json"))
	data, err := store.load()
	if err != nil {
		data = defaultSettingsStoreData()
	}
	return &SettingsService{store: store, data: data}
}

func (s *SettingsService) GetSettings() (AppSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AppSettings, nil
}

// updateAndPersist applies a modification to settings and persists to disk.
// On save failure, the modification is rolled back.
func (s *SettingsService) updateAndPersist(modify func(*settingsStoreData)) error {
	s.mu.Lock()
	prev := s.data
	modify(&s.data)
	s.data.normalize()
	data := s.data
	s.mu.Unlock()

	if err := s.store.save(data); err != nil {
		s.mu.Lock()
		s.data = prev
		s.mu.Unlock()
		return err
	}
	return nil
}

// updateAutoStartSetting keeps AutoStart side effects and persistence in sync.
// On save failure, the startup shortcut is rolled back to the previous state.
func (s *SettingsService) updateAutoStartSetting(nextAutoStart bool, modify func(*settingsStoreData)) error {
	s.mu.RLock()
	prevAutoStart := s.data.AppSettings.AutoStart
	s.mu.RUnlock()

	if prevAutoStart != nextAutoStart {
		if err := s.applyAutoStart(nextAutoStart); err != nil {
			return err
		}
	}

	if err := s.updateAndPersist(modify); err != nil {
		if prevAutoStart != nextAutoStart {
			_ = s.applyAutoStart(prevAutoStart)
		}
		return err
	}
	return nil
}

func escapePowerShellSingleQuoted(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func windowsStartupDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if strings.TrimSpace(appData) == "" {
		return "", errors.New("APPDATA is empty")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup"), nil
}

func (s *SettingsService) startupLinkPath() (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("autoStart is only supported on windows")
	}
	startupDir, err := windowsStartupDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(startupDir, "wincron.lnk"), nil
}

func (s *SettingsService) applyAutoStart(enabled bool) error {
	if runtime.GOOS != "windows" {
		return errors.New("autoStart is only supported on windows")
	}
	lnkPath, err := s.startupLinkPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lnkPath), 0o755); err != nil {
		return err
	}

	if !enabled {
		if err := os.Remove(lnkPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	if abs, err := filepath.Abs(exePath); err == nil {
		exePath = abs
	}
	workDir := filepath.Dir(exePath)

	psLnk := escapePowerShellSingleQuoted(lnkPath)
	psExe := escapePowerShellSingleQuoted(exePath)
	psWorkDir := escapePowerShellSingleQuoted(workDir)
	cmdText := fmt.Sprintf("$WshShell = New-Object -ComObject WScript.Shell; $Shortcut = $WshShell.CreateShortcut('%s'); $Shortcut.TargetPath = '%s'; $Shortcut.WorkingDirectory = '%s'; $Shortcut.Arguments = ''; $Shortcut.Save()", psLnk, psExe, psWorkDir)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", cmdText)
	attr := &syscall.SysProcAttr{}
	attrV := reflect.ValueOf(attr).Elem()
	if hide := attrV.FieldByName("HideWindow"); hide.IsValid() && hide.CanSet() && hide.Kind() == reflect.Bool {
		hide.SetBool(true)
	}
	cmd.SysProcAttr = attr
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create startup shortcut: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *SettingsService) SetSettings(settings AppSettings) error {
	return s.updateAutoStartSetting(settings.AutoStart, func(data *settingsStoreData) {
		data.AppSettings = settings
	})
}

func (s *SettingsService) SetSilentStart(enabled bool) error {
	return s.updateAndPersist(func(data *settingsStoreData) { data.AppSettings.SilentStart = enabled })
}

func (s *SettingsService) SetAutoStart(enabled bool) error {
	return s.updateAutoStartSetting(enabled, func(data *settingsStoreData) {
		data.AppSettings.AutoStart = enabled
	})
}

func (s *SettingsService) SetLightweightMode(enabled bool) error {
	return s.updateAndPersist(func(data *settingsStoreData) { data.AppSettings.LightweightMode = enabled })
}

func (s *SettingsService) SetRunInTray(enabled bool) error {
	return s.updateAndPersist(func(data *settingsStoreData) { data.AppSettings.RunInTray = enabled })
}

func (s *SettingsService) getRunInTray() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AppSettings.RunInTray
}

func (s *SettingsService) getLightweightMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AppSettings.LightweightMode
}

func (s *SettingsService) getSilentStart() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AppSettings.SilentStart
}

func (s *SettingsService) getAutoStart() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AppSettings.AutoStart
}

func (s *SettingsService) getWindowSize() (width int, height int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Local.WindowWidth, s.data.Local.WindowHeight
}

func (s *SettingsService) setWindowSize(width int, height int) error {
	width, height = normalizeWindowSize(width, height)
	if width == 0 || height == 0 {
		return nil
	}
	return s.updateAndPersist(func(data *settingsStoreData) {
		data.Local.WindowWidth = width
		data.Local.WindowHeight = height
	})
}

func (s *SettingsService) OpenDataDir() (string, error) {
	dir, err := resolveDataDir()
	if err != nil {
		return "", err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	return dir, nil
}

func (s *SettingsService) OpenEnvironmentVariables() error {
	if runtime.GOOS != "windows" {
		return errors.New("environment variables dialog is only supported on windows")
	}
	cmd := exec.Command("rundll32.exe", "sysdm.cpl,EditEnvironmentVariables")
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

// GetSystemBootTime returns the Windows system boot time as a formatted string
func GetSystemBootTime() string {
	// Use golang.org/x/sys/windows helper function
	uptime := windows.DurationSinceBoot()
	// Calculate boot time
	bootTime := time.Now().Add(-uptime)
	return bootTime.UTC().Format(time.RFC3339)
}

// GetLastSystemBootTime returns the stored last system boot time
func (s *SettingsService) GetLastSystemBootTime() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Local.LastSystemBootTime
}

// SetLastSystemBootTime updates the stored last system boot time
func (s *SettingsService) SetLastSystemBootTime(bootTime string) error {
	return s.updateAndPersist(func(data *settingsStoreData) { data.Local.LastSystemBootTime = bootTime })
}

// IsSystemRebooted checks if system has been rebooted since last run
func (s *SettingsService) IsSystemRebooted() bool {
	currentBootTime := GetSystemBootTime()
	lastBootTime := s.GetLastSystemBootTime()
	return currentBootTime != lastBootTime
}
