package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
}

func newLogStore(path string) *logStore {
	return &logStore{path: path}
}

func (s *logStore) append(entry JobLogEntry) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

 func (s *logStore) clear() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, []byte{}, 0o644)
 }

func (s *logStore) tail(jobID string, limit int) ([]JobLogEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []JobLogEntry{}, nil
		}
		return nil, err
	}
	defer f.Close()

	buf := make([]JobLogEntry, 0, limit)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e JobLogEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if jobID != "" && e.JobID != jobID {
			continue
		}
		if len(buf) == limit {
			copy(buf, buf[1:])
			buf[len(buf)-1] = e
			continue
		}
		buf = append(buf, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return buf, nil
}
