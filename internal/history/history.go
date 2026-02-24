package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"db-snap/internal/config"
	"db-snap/internal/model"
)

func Append(event model.AuditEvent) error {
	path, err := config.HistoryFile()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func List() ([]model.AuditEvent, error) {
	path, err := config.HistoryFile()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.AuditEvent{}, nil
		}
		return nil, err
	}
	defer f.Close()

	out := make([]model.AuditEvent, 0, 32)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Bytes()
		if len(line) == 0 {
			continue
		}
		var e model.AuditEvent
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("parse history entry: %w", err)
		}
		out = append(out, e)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
