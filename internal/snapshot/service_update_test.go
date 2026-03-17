package snapshot

import (
	"testing"
	"time"

	"db-snap/internal/model"
)

func TestNewSnapshotCreateSpecUsesCreateOptions(t *testing.T) {
	opts := CreateOptions{
		Tags:          []string{"baseline"},
		IncludeTables: []string{"public.users"},
		ExcludeTables: []string{"public.audit_logs"},
		Deterministic: true,
		Metadata:      map[string]string{"source": "cli"},
	}

	spec := newSnapshotCreateSpec(opts)
	if spec.ID == "" {
		t.Fatal("expected snapshot id")
	}
	if spec.CreatedAt.IsZero() {
		t.Fatal("expected created at")
	}
	if len(spec.Tags) != 1 || spec.Tags[0] != "baseline" {
		t.Fatalf("unexpected tags: %#v", spec.Tags)
	}
	if len(spec.IncludeTables) != 1 || spec.IncludeTables[0] != "public.users" {
		t.Fatalf("unexpected included tables: %#v", spec.IncludeTables)
	}
	if len(spec.ExcludeTables) != 1 || spec.ExcludeTables[0] != "public.audit_logs" {
		t.Fatalf("unexpected excluded tables: %#v", spec.ExcludeTables)
	}
	if !spec.Deterministic {
		t.Fatal("expected deterministic flag to be preserved")
	}
	if spec.Metadata["source"] != "cli" {
		t.Fatalf("unexpected metadata: %#v", spec.Metadata)
	}
	if spec.AuditEvent != "snapshot.create" {
		t.Fatalf("unexpected audit event: %s", spec.AuditEvent)
	}
	if spec.LogFileName != "create.log" {
		t.Fatalf("unexpected log file name: %s", spec.LogFileName)
	}

	opts.Tags[0] = "mutated"
	opts.IncludeTables[0] = "changed"
	opts.ExcludeTables[0] = "changed"
	opts.Metadata["source"] = "changed"
	if spec.Tags[0] != "baseline" || spec.IncludeTables[0] != "public.users" || spec.ExcludeTables[0] != "public.audit_logs" || spec.Metadata["source"] != "cli" {
		t.Fatal("expected spec to clone create options")
	}
}

func TestNewSnapshotUpdateSpecPreservesExistingManifest(t *testing.T) {
	createdAt := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	manifest := model.SnapshotManifest{
		ID:                 "snap-123",
		CreatedAt:          createdAt,
		Tags:               []string{"baseline"},
		FilteredTables:     []string{"public.users"},
		ExcludedTables:     []string{"public.audit_logs"},
		DeterministicOrder: true,
		Metadata:           map[string]string{"source": "tui"},
	}

	spec := newSnapshotUpdateSpec(manifest)
	if spec.ID != manifest.ID {
		t.Fatalf("expected id %s, got %s", manifest.ID, spec.ID)
	}
	if !spec.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected created at %s, got %s", createdAt, spec.CreatedAt)
	}
	if len(spec.Tags) != 1 || spec.Tags[0] != "baseline" {
		t.Fatalf("unexpected tags: %#v", spec.Tags)
	}
	if len(spec.IncludeTables) != 1 || spec.IncludeTables[0] != "public.users" {
		t.Fatalf("unexpected included tables: %#v", spec.IncludeTables)
	}
	if len(spec.ExcludeTables) != 1 || spec.ExcludeTables[0] != "public.audit_logs" {
		t.Fatalf("unexpected excluded tables: %#v", spec.ExcludeTables)
	}
	if !spec.Deterministic {
		t.Fatal("expected deterministic flag to be preserved")
	}
	if spec.Metadata["source"] != "tui" {
		t.Fatalf("unexpected metadata: %#v", spec.Metadata)
	}
	if spec.AuditEvent != "snapshot.update" {
		t.Fatalf("unexpected audit event: %s", spec.AuditEvent)
	}
	if spec.LogFileName != "update.log" {
		t.Fatalf("unexpected log file name: %s", spec.LogFileName)
	}

	manifest.Tags[0] = "mutated"
	manifest.FilteredTables[0] = "changed"
	manifest.ExcludedTables[0] = "changed"
	manifest.Metadata["source"] = "changed"
	if spec.Tags[0] != "baseline" || spec.IncludeTables[0] != "public.users" || spec.ExcludeTables[0] != "public.audit_logs" || spec.Metadata["source"] != "tui" {
		t.Fatal("expected spec to clone manifest values")
	}
}
