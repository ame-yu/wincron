package main

type Job struct {
	ID                      string   `json:"id" yaml:"id"`
	Name                    string   `json:"name" yaml:"name"`
	Cron                    string   `json:"cron" yaml:"cron"`
	Command                 string   `json:"command" yaml:"command"`
	Args                    []string `json:"args" yaml:"args"`
	WorkDir                 string   `json:"workDir" yaml:"workDir"`
	Console                 bool     `json:"console,omitempty" yaml:"console,omitempty"`
	ConcurrencyPolicy       string   `json:"concurrencyPolicy,omitempty" yaml:"concurrencyPolicy,omitempty"`
	Enabled                 bool     `json:"enabled" yaml:"enabled"`
	MaxConsecutiveFailures  int      `json:"maxConsecutiveFailures" yaml:"maxConsecutiveFailures"`
	ConsecutiveFailures     int      `json:"consecutiveFailures" yaml:"consecutiveFailures"`
	ExecutedCount           int      `json:"executedCount" yaml:"-"`
	LastExecutedAt          string   `json:"lastExecutedAt" yaml:"-"`
	NextRunAt               string   `json:"nextRunAt,omitempty" yaml:"-"`
}

type PreviewRunRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	WorkDir string   `json:"workDir"`
	Console bool `json:"console"`
	JobID   string   `json:"jobId"`
	JobName string   `json:"jobName"`
}

type JobLogEntry struct {
	ID         string `json:"id"`
	JobID      string `json:"jobId"`
	JobName    string `json:"jobName"`
	CommandLine string `json:"commandLine"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	ExitCode   int    `json:"exitCode"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Error      string `json:"error"`
}
