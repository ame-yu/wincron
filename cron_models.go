package main

type Job struct {
	ID                     string   `json:"id" yaml:"id"`
	Name                   string   `json:"name" yaml:"name"`
	Folder                 string   `json:"folder,omitempty" yaml:"folder,omitempty"`
	Cron                   string   `json:"cron" yaml:"cron"`
	Command                string   `json:"command" yaml:"command"`
	Args                   []string `json:"args" yaml:"args"`
	WorkDir                string   `json:"workDir" yaml:"workDir"`
	InheritEnv             *bool    `json:"inheritEnv,omitempty" yaml:"inheritEnv,omitempty"`
	Hotkey                 string   `json:"hotkey,omitempty" yaml:"hotkey,omitempty"`
	FlagProcessCreation    string   `json:"flagProcessCreation,omitempty" yaml:"flagProcessCreation,omitempty"`
	Timeout                int      `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	ConcurrencyPolicy      string   `json:"concurrencyPolicy,omitempty" yaml:"concurrencyPolicy,omitempty"`
	Enabled                bool     `json:"enabled" yaml:"enabled"`
	RunAtStartup           bool     `json:"runAtStartup" yaml:"runAtStartup"`
	MaxConsecutiveFailures int      `json:"maxConsecutiveFailures" yaml:"maxConsecutiveFailures"`
	ConsecutiveFailures    int      `json:"consecutiveFailures" yaml:"consecutiveFailures"`
	ExecutedCount          int      `json:"executedCount" yaml:"-"`
	LastExecutedAt         string   `json:"lastExecutedAt" yaml:"-"`
	NextRunAt              string   `json:"nextRunAt,omitempty" yaml:"-"`
}

type PreviewRunRequest struct {
	Command             string   `json:"command"`
	Args                []string `json:"args"`
	WorkDir             string   `json:"workDir"`
	InheritEnv          *bool    `json:"inheritEnv,omitempty"`
	FlagProcessCreation string   `json:"flagProcessCreation,omitempty"`
	Timeout             int      `json:"timeout"`
	JobID               string   `json:"jobId"`
	JobName             string   `json:"jobName"`
}

type JobLogEntry struct {
	ID            string `json:"id"`
	JobID         string `json:"jobId"`
	JobName       string `json:"jobName"`
	TriggerSource string `json:"triggerSource"`
	CommandLine   string `json:"commandLine"`
	StartedAt     string `json:"startedAt"`
	FinishedAt    string `json:"finishedAt"`
	ExitCode      int    `json:"exitCode"`
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	Error         string `json:"error"`
}

type JobLogPage struct {
	Items       []JobLogEntry `json:"items"`
	StoredCount int           `json:"storedCount"`
	TotalCount  int           `json:"totalCount"`
	HasMore     bool          `json:"hasMore"`
}
