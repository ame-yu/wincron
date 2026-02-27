package main

import (
	"bytes"
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

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const (
	flagCreateNewConsole = 0x00000010
	flagDetachedProcess  = 0x00000008
	flagCreateNoWindow   = 0x08000000
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
	running   map[string]map[string]*runningJobInstance
	globalEnabled bool
	hotkeys HotkeyManager
	hotkeysPaused bool
	onExecuted func(JobLogEntry)
}

type runningJobInstance struct {
	cmd *exec.Cmd
}

func applyJobWindowsProcessOptions(cmd *exec.Cmd, job Job) {
	if cmd == nil {
		return
	}
	if runtime.GOOS != "windows" {
		return
	}

	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	attrV := reflect.ValueOf(attr).Elem()

	setHideWindow := func(v bool) {
		hide := attrV.FieldByName("HideWindow")
		if hide.IsValid() && hide.CanSet() && hide.Kind() == reflect.Bool {
			hide.SetBool(v)
		}
	}

	updateCreationFlags := func(update func(current uint64) uint64) {
		cf := attrV.FieldByName("CreationFlags")
		if !cf.IsValid() || !cf.CanSet() {
			return
		}
		switch cf.Kind() {
		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			cf.SetUint(update(cf.Uint()))
		}
	}

	flag := normalizeProcessCreationFlag(job.FlagProcessCreation)
	createNewConsole := flag == "CREATE_NEW_CONSOLE"
	detachedProcess := flag == "DETACHED_PROCESS"
	createNoWindow := flag == "CREATE_NO_WINDOW"

	if createNewConsole {
		setHideWindow(false)
	} else if createNoWindow {
		setHideWindow(true)
	}
	updateCreationFlags(func(current uint64) uint64 {
		current &^= uint64(flagCreateNewConsole | flagDetachedProcess | flagCreateNoWindow)
		if createNewConsole {
			current |= uint64(flagCreateNewConsole)
		}
		if detachedProcess {
			current |= uint64(flagDetachedProcess)
		}
		if createNoWindow {
			current |= uint64(flagCreateNoWindow)
		}
		return current
	})

	cmd.SysProcAttr = attr
}

func renderCommandLine(command string, args []string) string {
	return strings.Join(append([]string{command}, args...), " ")
}

func normalizeProcessCreationFlag(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" || v == "CREATE_NEW_CONSOLE" || v == "CREATE_NO_WINDOW" || v == "DETACHED_PROCESS" {
		return v
	}
	return ""
}

func normalizeConcurrencyPolicy(policy string) string {
	v := strings.ToLower(strings.TrimSpace(policy))
	if v == "allow" || v == "kill_old" {
		return v
	}
	return "skip"
}

func isRebootCron(expr string) bool {
	return strings.EqualFold(strings.TrimSpace(expr), "@reboot")
}

func NewCronService() *CronService {
	baseDir := defaultDataDir()
	store := newJobStore(filepath.Join(baseDir, "jobs.json"))
	logs := newLogStore(filepath.Join(baseDir, "logs.sqlite"))

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	c := cron.New(cron.WithParser(parser))

	s := &CronService{
		jobs:      map[string]Job{},
		store:     store,
		logs:      logs,
		scheduler: c,
		parser:    parser,
		entries:   map[string]cron.EntryID{},
		running:   map[string]map[string]*runningJobInstance{},
		globalEnabled: true,
	}
	if runtime.GOOS == "windows" {
		s.hotkeys = newWindowsHotkeyManager(func(jobID string) {
			s.runHotkey(jobID)
		})
		s.hotkeys.Start()
	}

	s.reloadFromDisk()
	s.syncHotkeysFromJobs()
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


func (s *CronService) ListJobs() ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	jobs := make([]Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jj := j
		jj.NextRunAt = ""
		if jj.Enabled {
			jj.NextRunAt = s.computeNextRunAt(jj.ID, jj, now)
		}
		jobs = append(jobs, jj)
	}
	return jobs, nil
}

func (s *CronService) computeNextRunAt(jobID string, job Job, now time.Time) string {
	if jobID == "" {
		jobID = job.ID
	}
	if jobID == "" {
		return ""
	}

	// @reboot jobs show "At startup"
	if job.RunAtStartup && isRebootCron(job.Cron) {
		return "At startup"
	}

	if entryID, ok := s.entries[jobID]; ok {
		entry := s.scheduler.Entry(entryID)
		if !entry.Next.IsZero() {
			return entry.Next.Format(time.RFC3339)
		}
	}

	expr := strings.TrimSpace(job.Cron)
	if expr == "" {
		return ""
	}
	if schedule, err := s.parser.Parse(expr); err == nil {
		next := schedule.Next(now)
		if !next.IsZero() {
			return next.Format(time.RFC3339)
		}
	}
	return ""
}

 func (s *CronService) GetGlobalEnabled() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.globalEnabled, nil
 }

 func (s *CronService) SetGlobalEnabled(enabled bool) error {
	s.mu.Lock()
	manager := s.hotkeys
	paused := s.hotkeysPaused

	if s.globalEnabled == enabled {
		s.mu.Unlock()
		return nil
	}

	s.globalEnabled = enabled

	if !enabled {
		for _, entryID := range s.entries {
			s.scheduler.Remove(entryID)
		}
		s.entries = map[string]cron.EntryID{}
		s.mu.Unlock()
		if manager != nil {
			_ = manager.SetActive(false)
		}
		return nil
	}

	for id := range s.jobs {
		if err := s.rescheduleLocked(id); err != nil {
			s.mu.Unlock()
			return err
		}
	}
	s.mu.Unlock()
	if manager != nil {
		_ = manager.SetActive(!paused)
	}
	return nil
 }

func (s *CronService) stopHotkeys() {
	s.mu.Lock()
	manager := s.hotkeys
	s.hotkeys = nil
	s.mu.Unlock()
	if manager != nil {
		manager.Stop()
	}
}

func (s *CronService) syncHotkeysFromJobs() {
	s.mu.Lock()
	manager := s.hotkeys
	paused := s.hotkeysPaused
	globalEnabled := s.globalEnabled
	jobs := make([]Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	s.mu.Unlock()

	if manager == nil {
		return
	}

	for _, j := range jobs {
		hk := strings.TrimSpace(j.Hotkey)
		if hk != "" {
			normalized, _, _, err := normalizeHotkeyString(hk)
			if err != nil {
				_ = manager.SetBinding(j.ID, "")
				continue
			}
			hk = normalized
		}
		desired := ""
		if j.Enabled && hk != "" {
			desired = hk
		}
		_ = manager.SetBinding(j.ID, desired)
	}
	_ = manager.SetActive(globalEnabled && !paused)
}

func (s *CronService) runHotkey(id string) {
	s.runFromSource(id, "hotkey")
}

func (s *CronService) normalizeJobHotkeyLocked(jobID string, hotkey string) (string, error) {
	hk := strings.TrimSpace(hotkey)
	if hk == "" {
		return "", nil
	}
	normalized, _, _, err := normalizeHotkeyString(hk)
	if err != nil {
		return "", err
	}
	for id, j := range s.jobs {
		if id == jobID {
			continue
		}
		if strings.TrimSpace(j.Hotkey) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(j.Hotkey), normalized) {
			return "", errors.New("hotkey is already used by another job")
		}
	}
	return normalized, nil
}

func (s *CronService) ValidateJobHotkey(hotkey string) (string, error) {
	normalized, _, _, err := normalizeHotkeyString(hotkey)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

func (s *CronService) SetJobHotkey(id string, hotkey string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, errors.New("job not found")
	}

	normalized, err := s.normalizeJobHotkeyLocked(id, hotkey)
	if err != nil {
		return Job{}, err
	}

	job.Hotkey = normalized
	if err := s.setHotkeyBindingLocked(job); err != nil {
		return Job{}, err
	}
	s.jobs[id] = job
	if err := s.persistLocked(); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *CronService) PauseHotkeys() error {
	s.mu.Lock()
	s.hotkeysPaused = true
	manager := s.hotkeys
	s.mu.Unlock()
	if manager != nil {
		return manager.SetActive(false)
	}
	return nil
}

func (s *CronService) ResumeHotkeys() error {
	s.mu.Lock()
	s.hotkeysPaused = false
	manager := s.hotkeys
	globalEnabled := s.globalEnabled
	s.mu.Unlock()
	if manager != nil {
		return manager.SetActive(globalEnabled)
	}
	return nil
}

func (s *CronService) GetJobsWithHotkeys() ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	out := make([]Job, 0)
	for _, j := range s.jobs {
		if strings.TrimSpace(j.Hotkey) == "" {
			continue
		}
		jj := j
		jj.NextRunAt = ""
		if jj.Enabled {
			jj.NextRunAt = s.computeNextRunAt(jj.ID, jj, now)
		}
		out = append(out, jj)
	}
	return out, nil
}

func (s *CronService) PreviewNextRun(cronExpr string) (string, error) {
	expr := strings.TrimSpace(cronExpr)
	if expr == "" {
		return "", nil
	}

	// Handle @reboot
	if isRebootCron(expr) {
		return "At startup", nil
	}

	schedule, err := s.parser.Parse(expr)
	if err != nil {
		return "", fmt.Errorf("invalid cron: %w", err)
	}

	next := schedule.Next(time.Now())
	if next.IsZero() {
		return "", errors.New("failed to compute next run")
	}
	return next.Format(time.RFC3339), nil
}

func (s *CronService) UpsertJob(job Job) (Job, error) {
	job.NextRunAt = ""
	job.Cron = strings.TrimSpace(job.Cron)
	if job.Command == "" {
		return Job{}, errors.New("command is required")
	}
	if job.Timeout < 0 {
		job.Timeout = 0
	}
	if job.Name == "" {
		job.Name = job.Command
	}
	job.FlagProcessCreation = normalizeProcessCreationFlag(job.FlagProcessCreation)

	// Handle @reboot syntax - auto-set RunAtStartup
	if isRebootCron(job.Cron) {
		job.RunAtStartup = true
	} else {
		job.RunAtStartup = false
		if job.Cron != "" {
			if _, err := s.parser.Parse(job.Cron); err != nil {
				return Job{}, fmt.Errorf("invalid cron: %w", err)
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if normalized, err := s.normalizeJobHotkeyLocked(job.ID, job.Hotkey); err != nil {
		return Job{}, err
	} else {
		job.Hotkey = normalized
	}
	if prev, ok := s.jobs[job.ID]; ok {
		job.ConsecutiveFailures = prev.ConsecutiveFailures
		job.ExecutedCount = prev.ExecutedCount
		job.LastExecutedAt = prev.LastExecutedAt
		if strings.TrimSpace(job.ConcurrencyPolicy) == "" {
			job.ConcurrencyPolicy = prev.ConcurrencyPolicy
		}
		if job.MaxConsecutiveFailures < 0 {
			job.MaxConsecutiveFailures = 0
		}
	}
	job.ConcurrencyPolicy = normalizeConcurrencyPolicy(job.ConcurrencyPolicy)
	if err := s.setHotkeyBindingLocked(job); err != nil {
		return Job{}, err
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

func (s *CronService) setHotkeyBindingLocked(job Job) error {
	if s.hotkeys == nil {
		return nil
	}
	desired := ""
	if job.Enabled && strings.TrimSpace(job.Hotkey) != "" {
		desired = job.Hotkey
	}
	return s.hotkeys.SetBinding(job.ID, desired)
}

func (s *CronService) DeleteJob(id string) error {
	var manager HotkeyManager

	s.mu.Lock()
	manager = s.hotkeys
	delete(s.jobs, id)
	s.unscheduleLocked(id)
	err := s.persistLocked()
	s.mu.Unlock()

	if manager != nil {
		_ = manager.SetBinding(id, "")
	}
	return err
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
	}
	if err := s.setHotkeyBindingLocked(job); err != nil {
		return Job{}, err
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

func (s *CronService) SetJobFolder(id string, folder string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, errors.New("job not found")
	}
	job.Folder = strings.TrimSpace(folder)
	s.jobs[id] = job
	if err := s.persistLocked(); err != nil {
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
	entry, err := s.runJobWithPolicy(job, "manual")
	if err != nil {
		return JobLogEntry{}, err
	}
	if entry == nil {
		return JobLogEntry{}, errors.New("skipped")
	}
	s.logsMu.Lock()
	err = s.logs.append(*entry)
	s.logsMu.Unlock()
	if err != nil {
		return JobLogEntry{}, err
	}
	_ = s.applyExecutionResult(id, entry.ExitCode == 0, entry.FinishedAt)
	s.notifyExecuted(*entry)
	return *entry, nil
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
		FlagProcessCreation: normalizeProcessCreationFlag(req.FlagProcessCreation),
		Timeout: req.Timeout,
		Enabled: true,
	}

	entry := s.execute(job, "")
	s.logsMu.Lock()
	err := s.logs.append(entry)
	s.logsMu.Unlock()
	if err != nil {
		return JobLogEntry{}, err
	}
	s.notifyExecuted(entry)
	return entry, nil
}

func (s *CronService) runningCommands(jobID string) []*exec.Cmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	instances := s.running[jobID]
	cmds := make([]*exec.Cmd, 0, len(instances))
	for _, inst := range instances {
		if inst != nil && inst.cmd != nil && inst.cmd.Process != nil {
			cmds = append(cmds, inst.cmd)
		}
	}
	return cmds
}

func (s *CronService) reserveExecutionLocked(jobID string, policy string) (string, bool) {
	if policy == "skip" {
		if m := s.running[jobID]; len(m) > 0 {
			return "", true
		}
	}
	if s.running == nil {
		s.running = map[string]map[string]*runningJobInstance{}
	}
	instanceID := uuid.NewString()
	m := s.running[jobID]
	if m == nil {
		m = map[string]*runningJobInstance{}
		s.running[jobID] = m
	}
	m[instanceID] = &runningJobInstance{}
	return instanceID, false
}

func (s *CronService) runJobWithPolicy(job Job, source string) (*JobLogEntry, error) {
	job.ConcurrencyPolicy = normalizeConcurrencyPolicy(job.ConcurrencyPolicy)

	if job.ConcurrencyPolicy == "kill_old" {
		cmds := s.runningCommands(job.ID)
		for _, cmd := range cmds {
			if cmd == nil || cmd.Process == nil {
				continue
			}
			_ = cmd.Process.Kill()
		}
	}

	s.mu.Lock()
	instanceID, alreadyRunning := s.reserveExecutionLocked(job.ID, job.ConcurrencyPolicy)
	s.mu.Unlock()

	if alreadyRunning {
		if source == "manual" {
			return nil, errors.New("job is already running")
		}
		return nil, nil
	}

	entry := s.execute(job, instanceID)
	return &entry, nil
}


func (s *CronService) ListLogs(jobID string, limit int) ([]JobLogEntry, error) {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.tail(jobID, limit)
}

func (s *CronService) ClearLogs() error {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.clear()
}

func (s *CronService) ClearJobLogs(jobID string) error {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.clearJob(jobID)
}

func (s *CronService) DeleteLogEntry(entryID string) error {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.deleteEntry(entryID)
}

func (s *CronService) MergeLog(otherPath string) error {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.merge(otherPath)
}

func (s *CronService) reloadFromDisk() {
	jobs, err := s.store.load()
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, j := range jobs {
		j.NextRunAt = ""
		j.ConcurrencyPolicy = normalizeConcurrencyPolicy(j.ConcurrencyPolicy)
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

	expr := strings.TrimSpace(job.Cron)
	if expr == "" {
		return nil
	}

	// @reboot jobs don't need cron scheduling
	if isRebootCron(expr) {
		return nil
	}

	jobID := job.ID
	entryID, err := s.scheduler.AddFunc(expr, func() {
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

// RunStartupJobs executes all enabled jobs with RunAtStartup=true at application startup
func (s *CronService) RunStartupJobs() {
	s.mu.Lock()
	jobsToRun := make([]Job, 0)
	for _, job := range s.jobs {
		if job.Enabled && job.RunAtStartup && isRebootCron(job.Cron) {
			jobsToRun = append(jobsToRun, job)
		}
	}
	s.mu.Unlock()

	for _, job := range jobsToRun {
		go s.runScheduled(job.ID)
	}
}

func (s *CronService) runScheduled(id string) {
	s.runFromSource(id, "scheduled")
}

func (s *CronService) runFromSource(id, source string) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	globalEnabled := s.globalEnabled
	paused := s.hotkeysPaused
	s.mu.Unlock()
	if !ok || !globalEnabled || !job.Enabled {
		return
	}
	if source == "hotkey" && paused {
		return
	}
	entry, err := s.runJobWithPolicy(job, source)
	if err != nil || entry == nil {
		return
	}
	entryV := *entry
	_ = s.applyExecutionResult(id, entryV.ExitCode == 0, entryV.FinishedAt)
	s.logsMu.Lock()
	if err := s.logs.append(entryV); err == nil {
		s.notifyExecuted(entryV)
	}
	s.logsMu.Unlock()
}

func (s *CronService) applyExecutionResult(id string, ok bool, executedAt string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return nil
	}
	prevEnabled := job.Enabled
	job.ExecutedCount++
	job.LastExecutedAt = executedAt

	if ok {
		job.ConsecutiveFailures = 0
	} else {
		job.ConsecutiveFailures++
		// MaxConsecutiveFailures == 0 means no limit
		if job.Enabled && job.MaxConsecutiveFailures > 0 && job.ConsecutiveFailures >= job.MaxConsecutiveFailures {
			job.Enabled = false
		}
	}

	s.jobs[id] = job
	if err := s.persistLocked(); err != nil {
		return err
	}
	if job.Enabled != prevEnabled {
		return s.rescheduleLocked(id)
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

func (s *CronService) execute(job Job, runningInstanceID string) JobLogEntry {
	start := time.Now()

	cmd := exec.Command(job.Command, job.Args...)
	if job.WorkDir != "" {
		cmd.Dir = job.WorkDir
	}
	applyJobWindowsProcessOptions(cmd, job)

	if runningInstanceID != "" {
		s.mu.Lock()
		if instances, ok := s.running[job.ID]; ok {
			if inst, ok := instances[runningInstanceID]; ok {
				inst.cmd = cmd
			}
		}
		s.mu.Unlock()
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Start()
	timedOut := false
	if runErr == nil {
		if job.Timeout > 0 {
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()
			select {
			case runErr = <-done:
			case <-time.After(time.Duration(job.Timeout) * time.Second):
				timedOut = true
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				runErr = <-done
			}
		} else {
			runErr = cmd.Wait()
		}
	}

	exitCode := 0
	errText := ""
	if timedOut {
		exitCode = -1
		errText = fmt.Sprintf("timeout after %ds", job.Timeout)
	} else if runErr != nil {
		errText = runErr.Error()
		exitCode = -1
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			exitCode = ee.ExitCode()
		}
	}

	end := time.Now()

	if runningInstanceID != "" {
		s.mu.Lock()
		if instances, ok := s.running[job.ID]; ok {
			delete(instances, runningInstanceID)
			if len(instances) == 0 {
				delete(s.running, job.ID)
			}
		}
		s.mu.Unlock()
	}

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
