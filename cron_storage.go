package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type jobStore struct {
	path string
}

func newJobStore(path string) *jobStore {
	return &jobStore{path: path}
}

func (s *jobStore) load() ([]Job, error) {
	var jobs []Job
	err := readJSONOrDefault(s.path, &jobs, func() {
		jobs = []Job{}
	})
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *jobStore) save(jobs []Job) error {
	return writeJSONAtomic(s.path, jobs)
}

type logStore struct {
	path string
	db   *sql.DB

	initErr error

	insertStmt *sql.Stmt
}

func newLogStore(path string) *logStore {
	s := &logStore{path: path}
	if err := s.init(); err != nil {
		s.initErr = err
	}
	return s
}

func (s *logStore) append(entry JobLogEntry) error {
	if err := s.ensureInit(); err != nil {
		return err
	}
	if s.insertStmt == nil {
		return fmt.Errorf("log db not initialized")
	}

	startedAtMs := parseRFC3339ToUnixMs(entry.StartedAt)
	finishedAtMs := parseRFC3339ToUnixMs(entry.FinishedAt)

	_, err := s.insertStmt.Exec(
		entry.ID,
		entry.JobID,
		entry.JobName,
		entry.CommandLine,
		startedAtMs,
		finishedAtMs,
		entry.ExitCode,
		entry.Stdout,
		entry.Stderr,
		entry.Error,
	)
	return err
}

 func (s *logStore) clear() error {
	if err := s.ensureInit(); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM job_logs;`)
	return err
 }

func (s *logStore) clearJob(jobID string) error {
	if err := s.ensureInit(); err != nil {
		return err
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return fmt.Errorf("jobID is required")
	}
	_, err := s.db.Exec(`DELETE FROM job_logs WHERE job_id = ?;`, jobID)
	return err
}

func (s *logStore) deleteEntry(entryID string) error {
	if err := s.ensureInit(); err != nil {
		return err
	}
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return fmt.Errorf("entryID is required")
	}
	_, err := s.db.Exec(`DELETE FROM job_logs WHERE id = ?;`, entryID)
	return err
}

func (s *logStore) tail(jobID string, limit int) ([]JobLogEntry, error) {
	if err := s.ensureInit(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}

	var (
		rows *sql.Rows
		err  error
	)
	if strings.TrimSpace(jobID) == "" {
		rows, err = s.db.Query(
			`SELECT id, job_id, job_name, command_line, started_at, finished_at, exit_code, stdout, stderr, error
			 FROM job_logs
			 ORDER BY started_at DESC
			 LIMIT ?;`,
			limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, job_id, job_name, command_line, started_at, finished_at, exit_code, stdout, stderr, error
			 FROM job_logs
			 WHERE job_id = ?
			 ORDER BY started_at DESC
			 LIMIT ?;`,
			jobID,
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buf := make([]JobLogEntry, 0, limit)
	for rows.Next() {
		var (
			id          string
			jid         string
			jobName     string
			commandLine string
			startedAtMs int64
			finishedAtMs int64
			exitCode    int
			stdout      string
			stderr      string
			errText     string
		)
		if err := rows.Scan(
			&id,
			&jid,
			&jobName,
			&commandLine,
			&startedAtMs,
			&finishedAtMs,
			&exitCode,
			&stdout,
			&stderr,
			&errText,
		); err != nil {
			return nil, err
		}
		buf = append(buf, JobLogEntry{
			ID:          id,
			JobID:       jid,
			JobName:     jobName,
			CommandLine: commandLine,
			StartedAt:   unixMsToRFC3339(startedAtMs),
			FinishedAt:  unixMsToRFC3339(finishedAtMs),
			ExitCode:    exitCode,
			Stdout:      stdout,
			Stderr:      stderr,
			Error:       errText,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return buf, nil
}

func (s *logStore) merge(otherPath string) error {
	if err := s.ensureInit(); err != nil {
		return err
	}
	otherPath = strings.TrimSpace(otherPath)
	if otherPath == "" {
		return fmt.Errorf("otherPath is required")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	attachSQL := "ATTACH DATABASE " + quoteSQLiteString(otherPath) + " AS other;"
	if _, err := tx.Exec(attachSQL); err != nil {
		return err
	}
	defer func() {
		_, _ = tx.Exec("DETACH DATABASE other;")
	}()

	_, err = tx.Exec(`INSERT OR IGNORE INTO job_logs(
		id, job_id, job_name, command_line, started_at, finished_at, exit_code, stdout, stderr, error
	) SELECT
		id, job_id, job_name, command_line, started_at, finished_at, exit_code, stdout, stderr, error
	  FROM other.job_logs;`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *logStore) ensureInit() error {
	if s == nil {
		return fmt.Errorf("log store is nil")
	}
	if s.initErr != nil {
		return s.initErr
	}
	if s.db == nil {
		return fmt.Errorf("log db not initialized")
	}
	return nil
}

func (s *logStore) init() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return err
	}

	if _, err := db.Exec("PRAGMA busy_timeout=3000;"); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		_ = db.Close()
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS job_logs (
		id TEXT PRIMARY KEY,
		job_id TEXT NOT NULL,
		job_name TEXT NOT NULL,
		command_line TEXT NOT NULL,
		started_at INTEGER NOT NULL,
		finished_at INTEGER NOT NULL,
		exit_code INTEGER NOT NULL,
		stdout TEXT NOT NULL,
		stderr TEXT NOT NULL,
		error TEXT NOT NULL
	);`); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_job_logs_job_id_started_at ON job_logs(job_id, started_at DESC);`); err != nil {
		_ = db.Close()
		return err
	}

	insertStmt, err := db.Prepare(`INSERT INTO job_logs(
		id, job_id, job_name, command_line, started_at, finished_at, exit_code, stdout, stderr, error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		_ = db.Close()
		return err
	}

	s.db = db
	s.insertStmt = insertStmt
	return nil
}

func parseRFC3339ToUnixMs(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UnixMilli()
	}
	return 0
}

func unixMsToRFC3339(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).In(time.Local).Format(time.RFC3339)
}

func quoteSQLiteString(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
