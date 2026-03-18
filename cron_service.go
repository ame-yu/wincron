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
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const (
	flagCreateNewConsole = 0x00000010
	flagDetachedProcess  = 0x00000008
	flagCreateNoWindow   = 0x08000000
)

const (
	logTriggerSourceCron    = "cron"
	logTriggerSourceUI      = "ui"
	logTriggerSourceIPC     = "ipc"
	logTriggerSourceHotkey  = "hotkey"
	logTriggerSourcePreview = "preview"
)

type CronService struct {
	mu            sync.Mutex
	logsMu        sync.Mutex
	jobs          map[string]Job
	store         *jobStore
	logs          *logStore
	scheduler     *cron.Cron
	parser        cron.Parser
	entries       map[string]cron.EntryID
	running       map[string]map[string]*runningJobInstance
	globalEnabled bool
	hotkeys       HotkeyManager
	hotkeysPaused bool
	onStarted     func(JobLogEntry)
	onExecuted    func(JobLogEntry)
	onJobsChanged func()
}

type runningJobInstance struct {
	cmd   *exec.Cmd
	entry JobLogEntry
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

func normalizeLogTriggerSource(value string) string {
	if value = strings.ToLower(strings.TrimSpace(value)); value == logTriggerSourceCron || value == logTriggerSourceUI || value == logTriggerSourceIPC || value == logTriggerSourceHotkey || value == logTriggerSourcePreview {
		return value
	}
	return ""
}

func newRunningLogEntry(job Job, triggerSource string, startedAt time.Time) JobLogEntry {
	return JobLogEntry{
		ID:            uuid.NewString(),
		JobID:         job.ID,
		JobName:       job.Name,
		TriggerSource: normalizeLogTriggerSource(triggerSource),
		CommandLine:   renderCommandLine(job.Command, job.Args),
		StartedAt:     startedAt.Format(time.RFC3339),
	}
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
		jobs:          map[string]Job{},
		store:         store,
		logs:          logs,
		scheduler:     c,
		parser:        parser,
		entries:       map[string]cron.EntryID{},
		running:       map[string]map[string]*runningJobInstance{},
		globalEnabled: true,
	}
	if runtime.GOOS == "windows" {
		s.hotkeys = newWindowsHotkeyManager(func(jobID string) {
			s.runFromSource(jobID, logTriggerSourceHotkey)
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

func resolveDataDir() (string, error) {
	dir := defaultDataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return dir, nil
}

func resolveJobWorkDir(workDir string) (string, error) {
	dir := strings.TrimSpace(workDir)
	if dir != "" {
		return dir, nil
	}
	return resolveDataDir()
}

func (s *CronService) ListJobs() ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.snapshotJobsLocked(time.Now(), nil), nil
}

func (s *CronService) snapshotJobsLocked(now time.Time, include func(Job) bool) []Job {
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		if include != nil && !include(job) {
			continue
		}

		jj := job
		jj.NextRunAt = ""
		if jj.Enabled {
			jj.NextRunAt = s.computeNextRunAt(jj.ID, jj, now)
		}
		jobs = append(jobs, jj)
	}
	return jobs
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
		s.notifyJobsChanged()
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
	s.notifyJobsChanged()
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

	return s.snapshotJobsLocked(time.Now(), func(job Job) bool {
		return strings.TrimSpace(job.Hotkey) != ""
	}), nil
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
	if job.InheritEnv != nil && *job.InheritEnv {
		job.InheritEnv = nil
	}

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

	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if normalized, err := s.normalizeJobHotkeyLocked(job.ID, job.Hotkey); err != nil {
		s.mu.Unlock()
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
		s.mu.Unlock()
		return Job{}, err
	}
	s.jobs[job.ID] = job
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	if err := s.rescheduleLocked(job.ID); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	s.mu.Unlock()
	s.notifyJobsChanged()
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
	if err == nil {
		s.notifyJobsChanged()
	}
	return err
}

func (s *CronService) SetJobEnabled(id string, enabled bool) (Job, error) {
	s.mu.Lock()

	job, ok := s.jobs[id]
	if !ok {
		s.mu.Unlock()
		return Job{}, errors.New("job not found")
	}
	job.Enabled = enabled
	if enabled {
		job.ConsecutiveFailures = 0
	}
	if err := s.setHotkeyBindingLocked(job); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	s.jobs[id] = job
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	if err := s.rescheduleLocked(id); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	s.mu.Unlock()
	s.notifyJobsChanged()
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
	return s.runNow(id, logTriggerSourceUI)
}

func (s *CronService) runNow(id string, triggerSource string) (JobLogEntry, error) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	s.mu.Unlock()
	if !ok {
		return JobLogEntry{}, errors.New("job not found")
	}
	entry, err := s.runJobWithPolicy(job, triggerSource)
	if err != nil {
		return JobLogEntry{}, err
	}
	if entry == nil {
		return JobLogEntry{}, errors.New("skipped")
	}
	if err := s.finishExecution(id, *entry, true); err != nil {
		return JobLogEntry{}, err
	}
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
		ID:                  jobID,
		Name:                jobName,
		Command:             req.Command,
		Args:                req.Args,
		WorkDir:             req.WorkDir,
		InheritEnv:          req.InheritEnv,
		FlagProcessCreation: normalizeProcessCreationFlag(req.FlagProcessCreation),
		Timeout:             req.Timeout,
		Enabled:             true,
	}

	entry := s.execute(job, "", logTriggerSourcePreview)
	if err := s.finishExecution("", entry, false); err != nil {
		return JobLogEntry{}, err
	}
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

func (s *CronService) updateRunningInstance(jobID, instanceID string, update func(*runningJobInstance)) {
	if instanceID == "" {
		return
	}
	s.mu.Lock()
	if instances := s.running[jobID]; instances != nil {
		if inst := instances[instanceID]; inst != nil {
			update(inst)
		}
	}
	s.mu.Unlock()
}

func (s *CronService) runningLogEntries(jobID string) []JobLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	var entries []JobLogEntry
	for runningJobID, instances := range s.running {
		if jobID != "" && runningJobID != jobID {
			continue
		}
		for _, inst := range instances {
			if inst == nil || inst.entry.ID == "" {
				continue
			}
			entries = append(entries, inst.entry)
		}
	}
	return entries
}

func appendUniqueLogEntries(dst []JobLogEntry, seen map[string]struct{}, entries []JobLogEntry) []JobLogEntry {
	for _, entry := range entries {
		if entry.ID == "" {
			continue
		}
		if _, ok := seen[entry.ID]; ok {
			continue
		}
		seen[entry.ID] = struct{}{}
		dst = append(dst, entry)
	}
	return dst
}

func uniqueLogEntries(entries []JobLogEntry) []JobLogEntry {
	unique := make([]JobLogEntry, 0, len(entries))
	return appendUniqueLogEntries(unique, make(map[string]struct{}, len(entries)), entries)
}

func logEntryIDs(entries []JobLogEntry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.ID == "" {
			continue
		}
		ids = append(ids, entry.ID)
	}
	return ids
}

func (s *CronService) TerminateLogEntry(entryID string) error {
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return errors.New("log entry id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, instances := range s.running {
		for _, inst := range instances {
			if inst == nil || inst.entry.ID != entryID {
				continue
			}
			if inst.cmd == nil || inst.cmd.Process == nil {
				return errors.New("job process is not running")
			}
			return inst.cmd.Process.Kill()
		}
	}
	return errors.New("running job not found")
}

func (s *CronService) runJobWithPolicy(job Job, triggerSource string) (*JobLogEntry, error) {
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

	if alreadyRunning && (triggerSource == logTriggerSourceUI || triggerSource == logTriggerSourceIPC) {
		return nil, errors.New("job is already running")
	}
	if alreadyRunning {
		return nil, nil
	}

	entry := s.execute(job, instanceID, triggerSource)
	return &entry, nil
}

func (s *CronService) ListLogs(jobID string, limit int) ([]JobLogEntry, error) {
	s.logsMu.Lock()
	logs, err := s.logs.tail(jobID, limit)
	s.logsMu.Unlock()
	if err != nil {
		return nil, err
	}

	running := s.runningLogEntries(jobID)
	if len(running) == 0 {
		return logs, nil
	}

	combined := make([]JobLogEntry, 0, len(running)+len(logs))
	seen := make(map[string]struct{}, len(running)+len(logs))
	combined = appendUniqueLogEntries(combined, seen, running)
	combined = appendUniqueLogEntries(combined, seen, logs)
	return combined, nil
}

func (s *CronService) ListLogsPage(jobID string, offset int, limit int) (JobLogPage, error) {
	if offset < 0 {
		offset = 0
	}

	running := uniqueLogEntries(s.runningLogEntries(jobID))

	s.logsMu.Lock()
	logs, storedTotalCount, hasMore, err := s.logs.page(jobID, offset, limit)
	if err != nil {
		s.logsMu.Unlock()
		return JobLogPage{}, err
	}

	runningInStoreCount := 0
	if len(running) > 0 {
		runningInStoreCount, err = s.logs.countExistingIDs(jobID, logEntryIDs(running))
		if err != nil {
			s.logsMu.Unlock()
			return JobLogPage{}, err
		}
	}
	s.logsMu.Unlock()

	totalCount := storedTotalCount + len(running) - runningInStoreCount
	if totalCount < 0 {
		totalCount = 0
	}

	items := logs
	if offset == 0 && len(running) > 0 {
		items = make([]JobLogEntry, 0, len(running)+len(logs))
		seen := make(map[string]struct{}, len(running)+len(logs))
		items = appendUniqueLogEntries(items, seen, running)
		items = appendUniqueLogEntries(items, seen, logs)
	}

	return JobLogPage{
		Items:       items,
		StoredCount: len(logs),
		TotalCount:  totalCount,
		HasMore:     hasMore,
	}, nil
}

func (s *CronService) appendLog(entry JobLogEntry) error {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	return s.logs.append(entry)
}

func (s *CronService) finishExecution(jobID string, entry JobLogEntry, updateState bool) error {
	jobsChanged := false
	if updateState {
		if changed, err := s.applyExecutionResult(jobID, entry.ExitCode == 0, entry.FinishedAt); err == nil {
			jobsChanged = changed
		}
	}
	if err := s.appendLog(entry); err != nil {
		return err
	}
	if jobsChanged {
		s.notifyJobsChanged()
	}
	s.notifyExecuted(entry)
	return nil
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
		s.runFromSource(jobID, logTriggerSourceCron)
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
		go s.runFromSource(job.ID, logTriggerSourceCron)
	}
}

func (s *CronService) runFromSource(id, triggerSource string) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	globalEnabled := s.globalEnabled
	paused := s.hotkeysPaused
	s.mu.Unlock()
	if !ok || !globalEnabled || !job.Enabled {
		return
	}
	if triggerSource == logTriggerSourceHotkey && paused {
		return
	}
	entry, err := s.runJobWithPolicy(job, triggerSource)
	if triggerSource == logTriggerSourceCron {
		s.notifyJobsChanged()
	}
	if err != nil || entry == nil {
		return
	}
	entryV := *entry
	_ = s.finishExecution(id, entryV, true)
}

func (s *CronService) applyExecutionResult(id string, ok bool, executedAt string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return false, nil
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
		return false, err
	}
	if job.Enabled != prevEnabled {
		return true, s.rescheduleLocked(id)
	}
	return false, nil
}

func (s *CronService) setJobLogCallback(slot *func(JobLogEntry), f func(JobLogEntry)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	*slot = f
}

func (s *CronService) notifyJobLogCallback(slot *func(JobLogEntry), entry JobLogEntry) {
	s.mu.Lock()
	f := *slot
	s.mu.Unlock()
	if f != nil {
		go f(entry)
	}
}

func (s *CronService) setOnExecuted(f func(JobLogEntry)) { s.setJobLogCallback(&s.onExecuted, f) }

func (s *CronService) setOnStarted(f func(JobLogEntry)) { s.setJobLogCallback(&s.onStarted, f) }

func (s *CronService) setOnJobsChanged(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onJobsChanged = f
}

func (s *CronService) notifyStarted(entry JobLogEntry) { s.notifyJobLogCallback(&s.onStarted, entry) }

func (s *CronService) notifyExecuted(entry JobLogEntry) { s.notifyJobLogCallback(&s.onExecuted, entry) }

func (s *CronService) notifyJobsChanged() {
	s.mu.Lock()
	f := s.onJobsChanged
	s.mu.Unlock()
	if f == nil {
		return
	}
	go f()
}

func (s *CronService) execute(job Job, runningInstanceID string, triggerSource string) JobLogEntry {
	start := time.Now()
	entry := newRunningLogEntry(job, triggerSource, start)

	cmd := exec.Command(job.Command, job.Args...)
	workDir, err := resolveJobWorkDir(job.WorkDir)
	if err != nil {
		end := time.Now()
		entry.FinishedAt = end.Format(time.RFC3339)
		entry.ExitCode = -1
		entry.Error = err.Error()
		return entry
	}
	cmd.Dir = workDir
	inheritEnv := true
	if job.InheritEnv != nil {
		inheritEnv = *job.InheritEnv
	}
	if inheritEnv {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = []string{}
	}
	applyJobWindowsProcessOptions(cmd, job)

	s.updateRunningInstance(job.ID, runningInstanceID, func(inst *runningJobInstance) { inst.cmd = cmd })

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Start()
	timedOut := false
	if runErr == nil {
		s.updateRunningInstance(job.ID, runningInstanceID, func(inst *runningJobInstance) { inst.entry = entry })
		s.notifyStarted(entry)
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

	entry.FinishedAt = end.Format(time.RFC3339)
	entry.ExitCode = exitCode
	entry.Stdout = truncateString(outBuf.String(), 16*1024)
	entry.Stderr = truncateString(errBuf.String(), 16*1024)
	entry.Error = errText
	return entry
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
