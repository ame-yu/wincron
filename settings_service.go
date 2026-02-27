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
	WindowWidth   int    `json:"windowWidth,omitempty" yaml:"windowWidth,omitempty"`
	WindowHeight  int    `json:"windowHeight,omitempty" yaml:"windowHeight,omitempty"`
	LightweightMode bool `json:"lightweightMode,omitempty" yaml:"lightweightMode,omitempty"`
	SilentStart    bool `json:"silentStart,omitempty" yaml:"silentStart,omitempty"`
	AutoStart      bool `json:"autoStart,omitempty" yaml:"autoStart,omitempty"`
	RunInTray      bool `json:"runInTray" yaml:"runInTray"`
	LastSystemBootTime string `json:"lastSystemBootTime,omitempty" yaml:"lastSystemBootTime,omitempty"`
}

func defaultAppSettings() AppSettings {
	return AppSettings{
		RunInTray:       true,
		LightweightMode: true,
	}
}

type settingsStore struct {
	path string
}

func newSettingsStore(path string) *settingsStore {
	return &settingsStore{path: path}
}

func (s *settingsStore) load() (AppSettings, error) {
	settings := defaultAppSettings()
	if err := readJSONOrDefault(s.path, &settings, func() {}); err != nil {
		return AppSettings{}, err
	}
	if settings.WindowWidth < 200 || settings.WindowHeight < 200 {
		settings.WindowWidth = 0
		settings.WindowHeight = 0
	}
	return settings, nil
}

func (s *settingsStore) save(settings AppSettings) error {
	return writeJSONAtomic(s.path, settings)
}

type SettingsService struct {
	mu       sync.RWMutex
	store    *settingsStore
	settings AppSettings
}

func NewSettingsService() *SettingsService {
	baseDir := defaultDataDir()
	store := newSettingsStore(filepath.Join(baseDir, "settings.json"))
	settings, err := store.load()
	if err != nil {
		settings = defaultAppSettings()
	}
	return &SettingsService{store: store, settings: settings}
}

func (s *SettingsService) GetSettings() (AppSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings, nil
}

// updateAndPersist applies a modification to settings and persists to disk.
// On save failure, the modification is rolled back.
func (s *SettingsService) updateAndPersist(modify func(*AppSettings)) error {
	s.mu.Lock()
	prev := s.settings
	modify(&s.settings)
	settings := s.settings
	s.mu.Unlock()

	if err := s.store.save(settings); err != nil {
		s.mu.Lock()
		s.settings = prev
		s.mu.Unlock()
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
	if settings.WindowWidth < 200 || settings.WindowHeight < 200 {
		settings.WindowWidth = 0
		settings.WindowHeight = 0
	}

	s.mu.RLock()
	prev := s.settings
	s.mu.RUnlock()

	if prev.AutoStart != settings.AutoStart {
		if err := s.applyAutoStart(settings.AutoStart); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()

	if err := s.store.save(settings); err != nil {
		s.mu.Lock()
		s.settings = prev
		s.mu.Unlock()
		if prev.AutoStart != settings.AutoStart {
			_ = s.applyAutoStart(prev.AutoStart)
		}
		return err
	}
	return nil
 }

func (s *SettingsService) SetSilentStart(enabled bool) error {
	return s.updateAndPersist(func(st *AppSettings) { st.SilentStart = enabled })
}

func (s *SettingsService) SetAutoStart(enabled bool) error {
	s.mu.RLock()
	prevSettings := s.settings
	s.mu.RUnlock()

	if err := s.applyAutoStart(enabled); err != nil {
		return err
	}

	s.mu.Lock()
	prev := s.settings.AutoStart
	s.settings.AutoStart = enabled
	settings := s.settings
	s.mu.Unlock()

	if err := s.store.save(settings); err != nil {
		s.mu.Lock()
		s.settings.AutoStart = prev
		s.mu.Unlock()
		_ = s.applyAutoStart(prevSettings.AutoStart)
		return err
	}
	return nil
}

func (s *SettingsService) SetLightweightMode(enabled bool) error {
	return s.updateAndPersist(func(st *AppSettings) { st.LightweightMode = enabled })
}

func (s *SettingsService) SetRunInTray(enabled bool) error {
	return s.updateAndPersist(func(st *AppSettings) { st.RunInTray = enabled })
}

func (s *SettingsService) getRunInTray() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.RunInTray
}

func (s *SettingsService) getLightweightMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.LightweightMode
}

func (s *SettingsService) getSilentStart() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.SilentStart
}

func (s *SettingsService) getAutoStart() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.AutoStart
}

func (s *SettingsService) getWindowSize() (width int, height int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings.WindowWidth < 200 || s.settings.WindowHeight < 200 {
		return 0, 0
	}
	return s.settings.WindowWidth, s.settings.WindowHeight
}

func (s *SettingsService) setWindowSize(width int, height int) error {
	if width < 200 || height < 200 {
		return nil
	}
	return s.updateAndPersist(func(st *AppSettings) {
		st.WindowWidth = width
		st.WindowHeight = height
	})
}

func (s *SettingsService) OpenDataDir() (string, error) {
	dir := defaultDataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
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
	return s.settings.LastSystemBootTime
}

// SetLastSystemBootTime updates the stored last system boot time
func (s *SettingsService) SetLastSystemBootTime(bootTime string) error {
	return s.updateAndPersist(func(st *AppSettings) { st.LastSystemBootTime = bootTime })
}

// IsSystemRebooted checks if system has been rebooted since last run
func (s *SettingsService) IsSystemRebooted() bool {
	currentBootTime := GetSystemBootTime()
	lastBootTime := s.GetLastSystemBootTime()
	return currentBootTime != lastBootTime
}
