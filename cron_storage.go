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

	insertStmt      *sql.Stmt
	clearJobStmt    *sql.Stmt
	deleteEntryStmt *sql.Stmt
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
		normalizeLogTriggerSource(entry.TriggerSource),
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
	if s.clearJobStmt == nil {
		return fmt.Errorf("log db not initialized")
	}
	_, err := s.clearJobStmt.Exec(jobID)
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
	if s.deleteEntryStmt == nil {
		return fmt.Errorf("log db not initialized")
	}
	_, err := s.deleteEntryStmt.Exec(entryID)
	return err
}

func (s *logStore) tail(jobID string, limit int) ([]JobLogEntry, error) {
	logs, _, _, err := s.page(jobID, 0, limit)
	return logs, err
}

func (s *logStore) page(jobID string, offset int, limit int) ([]JobLogEntry, int, bool, error) {
	if err := s.ensureInit(); err != nil {
		return nil, 0, false, err
	}
	jobID = strings.TrimSpace(jobID)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100
	}

	totalCount, err := s.count(jobID)
	if err != nil {
		return nil, 0, false, err
	}
	if totalCount == 0 || offset >= totalCount {
		return []JobLogEntry{}, totalCount, false, nil
	}

	query := `SELECT id, job_id, job_name, trigger_source, command_line, started_at, finished_at, exit_code, stdout, stderr, error
		FROM job_logs`
	args := make([]any, 0, 3)
	if jobID != "" {
		query += `
		WHERE job_id = ?`
		args = append(args, jobID)
	}
	query += `
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?;`
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()

	buf := make([]JobLogEntry, 0, limit)
	for rows.Next() {
		var (
			id            string
			jid           string
			jobName       string
			triggerSource string
			commandLine   string
			startedAtMs   int64
			finishedAtMs  int64
			exitCode      int
			stdout        string
			stderr        string
			errText       string
		)
		if err := rows.Scan(
			&id,
			&jid,
			&jobName,
			&triggerSource,
			&commandLine,
			&startedAtMs,
			&finishedAtMs,
			&exitCode,
			&stdout,
			&stderr,
			&errText,
		); err != nil {
			return nil, 0, false, err
		}
		buf = append(buf, JobLogEntry{
			ID:            id,
			JobID:         jid,
			JobName:       jobName,
			TriggerSource: normalizeLogTriggerSource(triggerSource),
			CommandLine:   commandLine,
			StartedAt:     unixMsToRFC3339(startedAtMs),
			FinishedAt:    unixMsToRFC3339(finishedAtMs),
			ExitCode:      exitCode,
			Stdout:        stdout,
			Stderr:        stderr,
			Error:         errText,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}

	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	hasMore := offset+len(buf) < totalCount
	return buf, totalCount, hasMore, nil
}

func (s *logStore) count(jobID string) (int, error) {
	if err := s.ensureInit(); err != nil {
		return 0, err
	}
	jobID = strings.TrimSpace(jobID)

	var (
		count int
		err   error
	)
	if jobID == "" {
		err = s.db.QueryRow(`SELECT COUNT(1) FROM job_logs;`).Scan(&count)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(1) FROM job_logs WHERE job_id = ?;`, jobID).Scan(&count)
	}
	return count, err
}

func (s *logStore) countExistingIDs(jobID string, ids []string) (int, error) {
	if err := s.ensureInit(); err != nil {
		return 0, err
	}
	jobID = strings.TrimSpace(jobID)

	seen := make(map[string]struct{}, len(ids))
	args := make([]any, 0, len(ids)+1)
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		args = append(args, id)
	}
	if len(args) == 0 {
		return 0, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(args)), ",")
	query := `SELECT COUNT(1) FROM job_logs WHERE id IN (` + placeholders + `)`
	if jobID != "" {
		query += ` AND job_id = ?`
		args = append(args, jobID)
	}
	query += `;`

	var count int
	if err := s.db.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
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

	otherHasTriggerSource, err := hasSQLiteColumn(tx, "other", "job_logs", "trigger_source")
	if err != nil {
		return err
	}
	selectTriggerSource := "''"
	if otherHasTriggerSource {
		selectTriggerSource = "trigger_source"
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO job_logs(
		id, job_id, job_name, trigger_source, command_line, started_at, finished_at, exit_code, stdout, stderr, error
	) SELECT
		id, job_id, job_name, ` + selectTriggerSource + `, command_line, started_at, finished_at, exit_code, stdout, stderr, error
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
		trigger_source TEXT NOT NULL DEFAULT '',
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
	hasTriggerSource, err := hasSQLiteColumn(db, "", "job_logs", "trigger_source")
	if err != nil {
		_ = db.Close()
		return err
	}
	if !hasTriggerSource {
		if _, err := db.Exec(`ALTER TABLE job_logs ADD COLUMN trigger_source TEXT NOT NULL DEFAULT '';`); err != nil {
			_ = db.Close()
			return err
		}
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_job_logs_job_id_started_at ON job_logs(job_id, started_at DESC);`); err != nil {
		_ = db.Close()
		return err
	}

	insertStmt, err := db.Prepare(`INSERT INTO job_logs(
		id, job_id, job_name, trigger_source, command_line, started_at, finished_at, exit_code, stdout, stderr, error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		_ = db.Close()
		return err
	}

	clearJobStmt, err := db.Prepare(`DELETE FROM job_logs WHERE job_id = ?;`)
	if err != nil {
		_ = insertStmt.Close()
		_ = db.Close()
		return err
	}

	deleteEntryStmt, err := db.Prepare(`DELETE FROM job_logs WHERE id = ?;`)
	if err != nil {
		_ = clearJobStmt.Close()
		_ = insertStmt.Close()
		_ = db.Close()
		return err
	}

	s.db = db
	s.insertStmt = insertStmt
	s.clearJobStmt = clearJobStmt
	s.deleteEntryStmt = deleteEntryStmt
	return nil
}

func hasSQLiteColumn(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, schema string, table string, column string) (bool, error) {
	table = strings.TrimSpace(table)
	column = strings.TrimSpace(column)
	if table == "" || column == "" {
		return false, fmt.Errorf("table and column are required")
	}

	pragma := "PRAGMA table_info(" + table + ");"
	if schema = strings.TrimSpace(schema); schema != "" {
		pragma = "PRAGMA " + schema + ".table_info(" + table + ");"
	}

	rows, err := q.Query(pragma)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
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
