package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ConfigService struct {
	cron     *CronService
	settings *SettingsService
}

func NewConfigService(cron *CronService, settings *SettingsService) *ConfigService {
	return &ConfigService{cron: cron, settings: settings}
}

type exportedCronConfig struct {
	GlobalEnabled bool `yaml:"globalEnabled,omitempty"`
}

type exportedConfig struct {
	Version    int               `yaml:"version"`
	ExportedAt string            `yaml:"exportedAt,omitempty"`
	Settings   *AppSettings      `yaml:"settings,omitempty"`
	Cron       *exportedCronConfig `yaml:"cron,omitempty"`
	Jobs       []Job             `yaml:"jobs"`
}

func (s *ConfigService) ExportYAML(exportSettings bool, onlyEnabled bool) (string, error) {
	jobs, err := s.cron.ListJobs()
	if err != nil {
		return "", err
	}
	if onlyEnabled {
		filtered := make([]Job, 0, len(jobs))
		for _, j := range jobs {
			if j.Enabled {
				filtered = append(filtered, j)
			}
		}
		jobs = filtered
	}

	sort.Slice(jobs, func(i, j int) bool {
		ni := strings.ToLower(strings.TrimSpace(jobs[i].Name))
		nj := strings.ToLower(strings.TrimSpace(jobs[j].Name))
		if ni == nj {
			return jobs[i].ID < jobs[j].ID
		}
		return ni < nj
	})

	cfg := exportedConfig{
		Version:    1,
		ExportedAt: time.Now().Format(time.RFC3339),
		Jobs:       jobs,
	}

	if exportSettings {
		settings, err := s.settings.GetSettings()
		if err != nil {
			return "", err
		}
		globalEnabled, err := s.cron.GetGlobalEnabled()
		if err != nil {
			return "", err
		}
		cfg.Settings = &settings
		cfg.Cron = &exportedCronConfig{GlobalEnabled: globalEnabled}
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *ConfigService) ExportYAMLToFile(filePath string, exportSettings bool, onlyEnabled bool) (string, error) {
	if filePath == "" {
		return "", errors.New("filePath is required")
	}
	if abs, err := filepath.Abs(filePath); err == nil {
		filePath = abs
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".yml" && ext != ".yaml" {
		filePath += ".yml"
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return "", err
	}

	y, err := s.ExportYAML(exportSettings, onlyEnabled)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filePath, []byte(y), 0o644); err != nil {
		return "", err
	}
	return filePath, nil
}

func (s *ConfigService) CheckImportYAMLConflicts(yamlText string) ([]string, error) {
	jobs, _, _, err := parseYAMLConfig([]byte(yamlText))
	if err != nil {
		return nil, err
	}

	existing, err := s.cron.ListJobs()
	if err != nil {
		return nil, err
	}
	nameSet := make(map[string]struct{}, len(existing))
	for _, j := range existing {
		name := strings.TrimSpace(j.Name)
		if name == "" {
			name = strings.TrimSpace(j.Command)
		}
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}

	conflicts := make(map[string]struct{})
	for _, j := range jobs {
		name := strings.TrimSpace(j.Name)
		if name == "" {
			name = strings.TrimSpace(j.Command)
		}
		if name == "" {
			continue
		}
		if _, ok := nameSet[name]; ok {
			conflicts[name] = struct{}{}
		}
	}

	out := make([]string, 0, len(conflicts))
	for name := range conflicts {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func (s *ConfigService) ImportYAML(yamlText string, conflictStrategy string) error {
	jobs, settings, globalEnabled, err := parseYAMLConfig([]byte(yamlText))
	if err != nil {
		return err
	}

	strategy := strings.ToLower(strings.TrimSpace(conflictStrategy))
	if strategy == "" {
		strategy = "coexist"
	}
	if strategy != "coexist" && strategy != "overwrite" {
		return fmt.Errorf("invalid conflictStrategy: %s", conflictStrategy)
	}

	existing, err := s.cron.ListJobs()
	if err != nil {
		return err
	}
	existingByName := make(map[string]Job, len(existing))
	for _, j := range existing {
		name := strings.TrimSpace(j.Name)
		if name == "" {
			name = strings.TrimSpace(j.Command)
		}
		if name != "" {
			existingByName[name] = j
		}
	}

	reservedNames := make(map[string]struct{}, len(existingByName))
	for name := range existingByName {
		reservedNames[name] = struct{}{}
	}

	for _, raw := range jobs {
		job := raw
		job.ConsecutiveFailures = 0
		job.ID = ""

		name := strings.TrimSpace(job.Name)
		if name == "" {
			name = strings.TrimSpace(job.Command)
			job.Name = name
		}

		if name != "" {
			if existingJob, ok := existingByName[name]; ok {
				if strategy == "overwrite" {
					job.ID = existingJob.ID
				} else {
					job.Name = uniqueImportName(name, reservedNames)
				}
			}
		}

		if _, err := s.cron.UpsertJob(job); err != nil {
			return err
		}

		finalName := strings.TrimSpace(job.Name)
		if finalName != "" {
			reservedNames[finalName] = struct{}{}
		}
	}

	if settings != nil {
		if err := s.settings.SetSettings(*settings); err != nil {
			return err
		}
	}
	if globalEnabled != nil {
		if err := s.cron.SetGlobalEnabled(*globalEnabled); err != nil {
			return err
		}
	}
	return nil
}

func uniqueImportName(base string, reserved map[string]struct{}) string {
	candidate := base + " (imported)"
	if _, ok := reserved[candidate]; !ok {
		return candidate
	}
	for i := 2; i < 10000; i++ {
		c := fmt.Sprintf("%s (imported %d)", base, i)
		if _, ok := reserved[c]; !ok {
			return c
		}
	}
	return fmt.Sprintf("%s (imported %d)", base, time.Now().Unix())
}

func parseYAMLConfig(b []byte) (jobs []Job, settings *AppSettings, globalEnabled *bool, err error) {
	var list []Job
	if err0 := yaml.Unmarshal(b, &list); err0 == nil {
		return list, nil, nil, nil
	}

	var cfg exportedConfig
	if err = yaml.Unmarshal(b, &cfg); err != nil {
		return nil, nil, nil, err
	}
	jobs = cfg.Jobs
	settings = cfg.Settings
	if cfg.Cron != nil {
		ge := cfg.Cron.GlobalEnabled
		globalEnabled = &ge
	}
	return jobs, settings, globalEnabled, nil
}
