package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"db-snap/internal/model"
	"gopkg.in/yaml.v3"
)

func DefaultConfig() model.AppConfig {
	return model.AppConfig{
		Version: "1",
		Policy: model.SafetyPolicy{
			AllowCIDRs: []string{},
			DenyHostKeywords: []string{
				"prod", "production", "staging", "stage", "dev", "qa", "rds.amazonaws.com",
			},
			WarnEveryRestore: true,
		},
	}
}

func LoadConfig() (model.AppConfig, error) {
	if err := EnsureLayout(); err != nil {
		return model.AppConfig{}, err
	}
	path, err := ConfigFile()
	if err != nil {
		return model.AppConfig{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := SaveConfig(cfg); err != nil {
				return model.AppConfig{}, err
			}
			return cfg, nil
		}
		return model.AppConfig{}, err
	}
	var cfg model.AppConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return model.AppConfig{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Policy.DenyHostKeywords == nil {
		cfg.Policy.DenyHostKeywords = DefaultConfig().Policy.DenyHostKeywords
	}
	return cfg, nil
}

func SaveConfig(cfg model.AppConfig) error {
	if err := EnsureLayout(); err != nil {
		return err
	}
	path, err := ConfigFile()
	if err != nil {
		return err
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, b, 0o644)
}

func SaveProfile(p model.DBProfile) error {
	if err := EnsureLayout(); err != nil {
		return err
	}
	f, err := ProfileFile(p.Name)
	if err != nil {
		return err
	}
	b, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(f, b, 0o600)
}

func LoadProfile(name string) (model.DBProfile, error) {
	f, err := ProfileFile(name)
	if err != nil {
		return model.DBProfile{}, err
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return model.DBProfile{}, err
	}
	var p model.DBProfile
	if err := yaml.Unmarshal(b, &p); err != nil {
		return model.DBProfile{}, err
	}
	return p, nil
}

func ListProfiles() ([]model.DBProfile, error) {
	if err := EnsureLayout(); err != nil {
		return nil, err
	}
	base, err := HomeDir()
	if err != nil {
		return nil, err
	}
	d, err := os.ReadDir(filepath.Join(base, "profiles"))
	if err != nil {
		return nil, err
	}
	out := make([]model.DBProfile, 0, len(d))
	for _, entry := range d {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(base, "profiles", entry.Name()))
		if err != nil {
			return nil, err
		}
		var p model.DBProfile
		if err := yaml.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func DeleteProfile(name string) error {
	f, err := ProfileFile(name)
	if err != nil {
		return err
	}
	if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func SaveRulePack(r model.RulePack) error {
	f, err := RulesFile(r.Profile)
	if err != nil {
		return err
	}
	b, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(f, b, 0o600)
}

func LoadRulePack(profile string) (model.RulePack, error) {
	f, err := RulesFile(profile)
	if err != nil {
		return model.RulePack{}, err
	}
	b, err := os.ReadFile(f)
	if err != nil {
		if os.IsNotExist(err) {
			return model.RulePack{Profile: profile, Rules: []model.ColumnRule{}}, nil
		}
		return model.RulePack{}, err
	}
	var r model.RulePack
	if err := yaml.Unmarshal(b, &r); err != nil {
		return model.RulePack{}, err
	}
	return r, nil
}

func SaveManifest(profile string, m model.SnapshotManifest) error {
	d, err := SnapshotDir(profile, m.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "manifest.json"), b, 0o644)
}

func LoadManifest(profile, snapshotID string) (model.SnapshotManifest, error) {
	d, err := SnapshotDir(profile, snapshotID)
	if err != nil {
		return model.SnapshotManifest{}, err
	}
	b, err := os.ReadFile(filepath.Join(d, "manifest.json"))
	if err != nil {
		return model.SnapshotManifest{}, err
	}
	var m model.SnapshotManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return model.SnapshotManifest{}, err
	}
	return m, nil
}

func SaveSchemaFingerprint(profile, snapshotID string, sf model.SchemaFingerprint) error {
	d, err := SnapshotDir(profile, snapshotID)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "schema_fingerprint.json"), b, 0o644)
}

func LoadSchemaFingerprint(profile, snapshotID string) (model.SchemaFingerprint, error) {
	d, err := SnapshotDir(profile, snapshotID)
	if err != nil {
		return model.SchemaFingerprint{}, err
	}
	b, err := os.ReadFile(filepath.Join(d, "schema_fingerprint.json"))
	if err != nil {
		return model.SchemaFingerprint{}, err
	}
	var sf model.SchemaFingerprint
	if err := json.Unmarshal(b, &sf); err != nil {
		return model.SchemaFingerprint{}, err
	}
	return sf, nil
}

func ListSnapshots(profile string) ([]model.SnapshotManifest, error) {
	dir, err := SnapshotProfileDir(profile)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.SnapshotManifest{}, nil
		}
		return nil, err
	}
	out := make([]model.SnapshotManifest, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := LoadManifest(profile, e.Name())
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

type activeSnapshotState struct {
	Profile    string    `json:"profile"`
	SnapshotID string    `json:"snapshotId"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func SaveActiveSnapshot(profile, snapshotID string) error {
	if err := EnsureLayout(); err != nil {
		return err
	}
	f, err := ActiveSnapshotFile(profile)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(activeSnapshotState{
		Profile:    profile,
		SnapshotID: snapshotID,
		UpdatedAt:  time.Now().UTC(),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f, b, 0o644)
}

func LoadActiveSnapshot(profile string) (string, error) {
	f, err := ActiveSnapshotFile(profile)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(f)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var s activeSnapshotState
	if err := json.Unmarshal(b, &s); err != nil {
		return "", err
	}
	return s.SnapshotID, nil
}

func DeleteSnapshot(profile, id string) error {
	d, err := SnapshotDir(profile, id)
	if err != nil {
		return err
	}
	return os.RemoveAll(d)
}
