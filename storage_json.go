package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func readJSONOrDefault(path string, dst any, setDefault func()) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			setDefault()
			return nil
		}
		return err
	}
	if len(b) == 0 {
		setDefault()
		return nil
	}
	return json.Unmarshal(b, dst)
}

func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
