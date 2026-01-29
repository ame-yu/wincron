package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type CronService struct {
	mu        sync.Mutex
	logsMu    sync.Mutex
	jobs      map[string]Job
	store     *jobStore
	logs      *logStore
	scheduler *cron.Cron
	parser    cron.Parser
	entries   map[string]cron.EntryID
	globalEnabled bool
	onExecuted func(JobLogEntry)
}

func renderCommandLine(command string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, command)
	for _, a := range args {
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

func NewCronService() *CronService {
	baseDir := defaultDataDir()
	store := newJobStore(filepath.Join(baseDir, "jobs.json"))
	logs := newLogStore(filepath.Join(baseDir, "logs.jsonl"))

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	c := cron.New(cron.WithParser(parser))

	s := &CronService{
		jobs:      map[string]Job{},
		store:     store,
		logs:      logs,
		scheduler: c,
		parser:    parser,
		entries:   map[string]cron.EntryID{},
		globalEnabled: true,
	}

	s.reloadFromDisk()
	s.scheduler.Start()
	return s
}

func defaultDataDir() string {
	if pd := os.Getenv("ProgramData"); pd != "" {
		return filepath.Join(pd, "wincron")
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "wincron")
	}
	return filepath.Join(".", "data")
}

func (s *CronService) appendLog(entry JobLogEntry) error {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.append(entry)
}

func (s *CronService) tailLogs(jobID string, limit int) ([]JobLogEntry, error) {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.tail(jobID, limit)
}

func (s *CronService) clearLogs() error {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.clear()
}

func (s *CronService) ListJobs() ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs := make([]Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	return jobs, nil
}

 func (s *CronService) GetGlobalEnabled() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.globalEnabled, nil
 }

 func (s *CronService) SetGlobalEnabled(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.globalEnabled == enabled {
		return nil
	}

	s.globalEnabled = enabled

	if !enabled {
		for _, entryID := range s.entries {
			s.scheduler.Remove(entryID)
		}
		s.entries = map[string]cron.EntryID{}
		return nil
	}

	for id := range s.jobs {
		if err := s.rescheduleLocked(id); err != nil {
			return err
		}
	}
	return nil
 }

func (s *CronService) UpsertJob(job Job) (Job, error) {
	if job.Cron == "" {
		return Job{}, errors.New("cron is required")
	}
	if job.Command == "" {
		return Job{}, errors.New("command is required")
	}
	if job.Name == "" {
		job.Name = job.Command
	}

	if _, err := s.parser.Parse(job.Cron); err != nil {
		return Job{}, fmt.Errorf("invalid cron: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if prev, ok := s.jobs[job.ID]; ok {
		job.ConsecutiveFailures = prev.ConsecutiveFailures
		if job.MaxConsecutiveFailures <= 0 {
			job.MaxConsecutiveFailures = prev.MaxConsecutiveFailures
		}
	}
	if job.MaxConsecutiveFailures <= 0 {
		job.MaxConsecutiveFailures = 3
	}
	s.jobs[job.ID] = job
	if err := s.persistLocked(); err != nil {
		return Job{}, err
	}
	if err := s.rescheduleLocked(job.ID); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *CronService) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.jobs, id)
	s.unscheduleLocked(id)
	return s.persistLocked()
}

func (s *CronService) SetJobEnabled(id string, enabled bool) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, errors.New("job not found")
	}
	job.Enabled = enabled
	if enabled {
		job.ConsecutiveFailures = 0
		if job.MaxConsecutiveFailures <= 0 {
			job.MaxConsecutiveFailures = 3
		}
	}
	s.jobs[id] = job
	if err := s.persistLocked(); err != nil {
		return Job{}, err
	}
	if err := s.rescheduleLocked(id); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *CronService) RunNow(id string) (JobLogEntry, error) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	s.mu.Unlock()
	if !ok {
		return JobLogEntry{}, errors.New("job not found")
	}
	entry := s.execute(job)
	_ = s.applyExecutionResult(id, entry.ExitCode == 0)
	if err := s.appendLog(entry); err != nil {
		return JobLogEntry{}, err
	}
	s.notifyExecuted(entry)
	return entry, nil
}

func (s *CronService) RunPreview(req PreviewRunRequest) (JobLogEntry, error) {
	if req.Command == "" {
		return JobLogEntry{}, errors.New("command is required")
	}

	jobID := req.JobID
	if jobID == "" {
		jobID = "preview-" + uuid.NewString()
	}

	jobName := req.JobName
	if jobName == "" {
		jobName = req.Command
	}

	job := Job{
		ID:      jobID,
		Name:    jobName,
		Command: req.Command,
		Args:    req.Args,
		WorkDir: req.WorkDir,
		Enabled: true,
	}

	entry := s.execute(job)
	if err := s.appendLog(entry); err != nil {
		return JobLogEntry{}, err
	}
	s.notifyExecuted(entry)
	return entry, nil
}

func (s *CronService) ListLogs(jobID string, limit int) ([]JobLogEntry, error) {
	return s.tailLogs(jobID, limit)
}

func (s *CronService) ClearLogs() error {
	return s.clearLogs()
}

func (s *CronService) ResetAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entryID := range s.entries {
		s.scheduler.Remove(entryID)
	}
	s.entries = map[string]cron.EntryID{}
	s.jobs = map[string]Job{}

	if err := s.store.save([]Job{}); err != nil {
		return err
	}
	return s.clearLogs()
}

func (s *CronService) reloadFromDisk() {
	jobs, err := s.store.load()
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, j := range jobs {
		if j.MaxConsecutiveFailures <= 0 {
			j.MaxConsecutiveFailures = 3
		}
		s.jobs[j.ID] = j
		_ = s.rescheduleLocked(j.ID)
	}
}

func (s *CronService) persistLocked() error {
	jobs := make([]Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	return s.store.save(jobs)
}

func (s *CronService) rescheduleLocked(id string) error {
	s.unscheduleLocked(id)

	job, ok := s.jobs[id]
	if !ok {
		return nil
	}
	if !s.globalEnabled {
		return nil
	}
	if !job.Enabled {
		return nil
	}

	jobID := job.ID
	entryID, err := s.scheduler.AddFunc(job.Cron, func() {
		s.runScheduled(jobID)
	})
	if err != nil {
		return err
	}
	s.entries[id] = entryID
	return nil
}

func (s *CronService) unscheduleLocked(id string) {
	entryID, ok := s.entries[id]
	if !ok {
		return
	}
	s.scheduler.Remove(entryID)
	delete(s.entries, id)
}

func (s *CronService) runScheduled(id string) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	globalEnabled := s.globalEnabled
	s.mu.Unlock()
	if !ok {
		return
	}
	if !globalEnabled {
		return
	}
	if !job.Enabled {
		return
	}
	entry := s.execute(job)
	_ = s.applyExecutionResult(id, entry.ExitCode == 0)
	if err := s.appendLog(entry); err == nil {
		s.notifyExecuted(entry)
	}
}

 func (s *CronService) applyExecutionResult(id string, ok bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return nil
	}
	if job.MaxConsecutiveFailures <= 0 {
		job.MaxConsecutiveFailures = 3
	}

	prevEnabled := job.Enabled
	prevFailures := job.ConsecutiveFailures
	prevMax := job.MaxConsecutiveFailures

	if ok {
		job.ConsecutiveFailures = 0
	} else {
		job.ConsecutiveFailures++
		if job.Enabled && job.ConsecutiveFailures >= job.MaxConsecutiveFailures {
			job.Enabled = false
		}
	}

	changed := job.Enabled != prevEnabled || job.ConsecutiveFailures != prevFailures || job.MaxConsecutiveFailures != prevMax
	if !changed {
		return nil
	}

	s.jobs[id] = job
	if err := s.persistLocked(); err != nil {
		return err
	}
	if job.Enabled != prevEnabled {
		if err := s.rescheduleLocked(id); err != nil {
			return err
		}
	}
	return nil
 }

func (s *CronService) setOnExecuted(f func(JobLogEntry)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onExecuted = f
}

func (s *CronService) notifyExecuted(entry JobLogEntry) {
	s.mu.Lock()
	f := s.onExecuted
	s.mu.Unlock()
	if f == nil {
		return
	}
	go f(entry)
}

func (s *CronService) execute(job Job) JobLogEntry {
	start := time.Now()

	cmd := exec.Command(job.Command, job.Args...)
	if job.WorkDir != "" {
		cmd.Dir = job.WorkDir
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	exitCode := 0
	errText := ""
	if runErr != nil {
		errText = runErr.Error()
		exitCode = -1
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			exitCode = ee.ExitCode()
		}
	}

	end := time.Now()

	stdout := truncateString(outBuf.String(), 16*1024)
	stderr := truncateString(errBuf.String(), 16*1024)

	return JobLogEntry{
		ID:         uuid.NewString(),
		JobID:      job.ID,
		JobName:    job.Name,
		CommandLine: renderCommandLine(job.Command, job.Args),
		StartedAt:  start.Format(time.RFC3339),
		FinishedAt: end.Format(time.RFC3339),
		ExitCode:   exitCode,
		Stdout:     stdout,
		Stderr:     stderr,
		Error:      errText,
	}
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
}
