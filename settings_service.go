package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"syscall"
	"strings"
	"sync"
)

const (
	CloseBehaviorExit = "exit"
	CloseBehaviorTray = "tray"
)

type AppSettings struct {
	CloseBehavior string `json:"closeBehavior" yaml:"closeBehavior"`
	WindowWidth   int    `json:"windowWidth,omitempty" yaml:"windowWidth,omitempty"`
	WindowHeight  int    `json:"windowHeight,omitempty" yaml:"windowHeight,omitempty"`
	LightweightMode bool `json:"lightweightMode,omitempty" yaml:"lightweightMode,omitempty"`
	SilentStart    bool `json:"silentStart,omitempty" yaml:"silentStart,omitempty"`
	AutoStart      bool `json:"autoStart,omitempty" yaml:"autoStart,omitempty"`
}

func defaultAppSettings() AppSettings {
	return AppSettings{
		CloseBehavior: CloseBehaviorTray,
	}
}

type settingsStore struct {
	path string
}

func newSettingsStore(path string) *settingsStore {
	return &settingsStore{path: path}
}

func (s *settingsStore) load() (AppSettings, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultAppSettings(), nil
		}
		return AppSettings{}, err
	}
	if len(b) == 0 {
		return defaultAppSettings(), nil
	}
	var settings AppSettings
	if err := json.Unmarshal(b, &settings); err != nil {
		return AppSettings{}, err
	}
	if settings.CloseBehavior != CloseBehaviorExit && settings.CloseBehavior != CloseBehaviorTray {
		settings.CloseBehavior = defaultAppSettings().CloseBehavior
	}
	if settings.WindowWidth < 200 || settings.WindowHeight < 200 {
		settings.WindowWidth = 0
		settings.WindowHeight = 0
	}
	return settings, nil
}

func (s *settingsStore) save(settings AppSettings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
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
	if settings.CloseBehavior != CloseBehaviorExit && settings.CloseBehavior != CloseBehaviorTray {
		settings.CloseBehavior = defaultAppSettings().CloseBehavior
	}
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

func (s *SettingsService) SetCloseBehavior(behavior string) error {
	if behavior != CloseBehaviorExit && behavior != CloseBehaviorTray {
		return errors.New("invalid closeBehavior")
	}

	s.mu.Lock()
	prev := s.settings.CloseBehavior
	s.settings.CloseBehavior = behavior
	settings := s.settings
	s.mu.Unlock()

	if err := s.store.save(settings); err != nil {
		s.mu.Lock()
		s.settings.CloseBehavior = prev
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *SettingsService) SetSilentStart(enabled bool) error {
	s.mu.Lock()
	prev := s.settings.SilentStart
	s.settings.SilentStart = enabled
	settings := s.settings
	s.mu.Unlock()

	if err := s.store.save(settings); err != nil {
		s.mu.Lock()
		s.settings.SilentStart = prev
		s.mu.Unlock()
		return err
	}
	return nil
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
	s.mu.Lock()
	prev := s.settings.LightweightMode
	s.settings.LightweightMode = enabled
	settings := s.settings
	s.mu.Unlock()

	if err := s.store.save(settings); err != nil {
		s.mu.Lock()
		s.settings.LightweightMode = prev
		s.mu.Unlock()
		return err
	}
	return nil
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

func (s *SettingsService) getCloseBehavior() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings.CloseBehavior != CloseBehaviorExit && s.settings.CloseBehavior != CloseBehaviorTray {
		return defaultAppSettings().CloseBehavior
	}
	return s.settings.CloseBehavior
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

	s.mu.Lock()
	prevW := s.settings.WindowWidth
	prevH := s.settings.WindowHeight
	s.settings.WindowWidth = width
	s.settings.WindowHeight = height
	settings := s.settings
	s.mu.Unlock()

	if err := s.store.save(settings); err != nil {
		s.mu.Lock()
		s.settings.WindowWidth = prevW
		s.settings.WindowHeight = prevH
		s.mu.Unlock()
		return err
	}
	return nil
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
