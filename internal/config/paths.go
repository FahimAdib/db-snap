package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func HomeDir() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".db-snap"), nil
}

func EnsureLayout() error {
	base, err := HomeDir()
	if err != nil {
		return err
	}
	dirs := []string{
		base,
		filepath.Join(base, "profiles"),
		filepath.Join(base, "rules"),
		filepath.Join(base, "snapshots"),
		filepath.Join(base, "history"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	return nil
}

func ConfigFile() (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.yaml"), nil
}

func ProfileFile(name string) (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "profiles", name+".yaml"), nil
}

func RulesFile(profile string) (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rules", profile+".yaml"), nil
}

func SnapshotDir(profile, snapshotID string) (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "snapshots", profile, snapshotID), nil
}

func SnapshotProfileDir(profile string) (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "snapshots", profile), nil
}

func HistoryFile() (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "history", "events.jsonl"), nil
}
