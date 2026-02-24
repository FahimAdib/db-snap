package snapshot

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"db-snap/internal/archive"
	"db-snap/internal/autofill"
	"db-snap/internal/config"
	"db-snap/internal/db"
	"db-snap/internal/drift"
	"db-snap/internal/history"
	"db-snap/internal/model"
	"db-snap/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type Service struct {
	AppVersion string
}

type CreateOptions struct {
	Profile       string
	Tags          []string
	IncludeTables []string
	ExcludeTables []string
	Deterministic bool
	Metadata      map[string]string
}

type RestorePlan struct {
	Compatible     bool
	RequiresPrompt []model.ColumnSummary
	Diff           model.SchemaDiff
	Warnings       []string
}

type PromptFunc func([]model.ColumnSummary) (map[string]string, error)

var insertRegex = regexp.MustCompile(`(?is)^insert\s+into\s+((?:"[^"]+"|[a-zA-Z0-9_]+)\.)?((?:"[^"]+"|[a-zA-Z0-9_]+))\s*\((.+)\)\s*values\s*\((.+)\);?$`)

func (s Service) Export(profile, snapshotID, outFile string) error {
	dir, err := config.SnapshotDir(profile, snapshotID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		return err
	}
	if outFile == "" {
		outFile = fmt.Sprintf("%s-%s.tar.gz", profile, snapshotID)
	}
	if err := archive.TarGzDir(dir, outFile); err != nil {
		return err
	}
	_ = history.Append(model.AuditEvent{ID: uuid.NewString(), Type: "snapshot.export", Profile: profile, Snapshot: snapshotID, CreatedAt: time.Now().UTC(), Status: "success", Details: map[string]string{"file": outFile}})
	return nil
}

func (s Service) Import(profile, srcFile string) (string, error) {
	tmp, err := os.MkdirTemp("", "db-snap-import-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := archive.UntarGz(tmp, srcFile); err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(tmp, "manifest.json"))
	if err != nil {
		return "", err
	}
	var m model.SnapshotManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return "", err
	}
	if m.ID == "" {
		return "", errors.New("imported snapshot missing id")
	}
	targetDir, err := config.SnapshotDir(profile, m.ID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	if err := copyDir(tmp, targetDir); err != nil {
		return "", err
	}
	m.Profile = profile
	if err := config.SaveManifest(profile, m); err != nil {
		return "", err
	}
	_ = history.Append(model.AuditEvent{ID: uuid.NewString(), Type: "snapshot.import", Profile: profile, Snapshot: m.ID, CreatedAt: time.Now().UTC(), Status: "success", Details: map[string]string{"file": srcFile}})
	return m.ID, nil
}

func (s Service) Create(ctx context.Context, opts CreateOptions) (model.SnapshotManifest, error) {
	if opts.Profile == "" {
		return model.SnapshotManifest{}, errors.New("profile is required")
	}
	if err := config.EnsureLayout(); err != nil {
		return model.SnapshotManifest{}, err
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return model.SnapshotManifest{}, err
	}
	p, err := config.LoadProfile(opts.Profile)
	if err != nil {
		return model.SnapshotManifest{}, err
	}
	if _, err := policy.ValidateTarget(p.Host, cfg.Policy); err != nil {
		return model.SnapshotManifest{}, err
	}

	password, _ := config.LoadPassword(p.Name)
	pool, err := db.OpenPool(ctx, p, password)
	if err != nil {
		return model.SnapshotManifest{}, err
	}
	defer pool.Close()

	sf, hash, err := db.CaptureSchemaFingerprint(ctx, pool)
	if err != nil {
		return model.SnapshotManifest{}, err
	}
	counts, err := db.TableRowCounts(ctx, pool)
	if err != nil {
		return model.SnapshotManifest{}, err
	}

	id := time.Now().UTC().Format("20060102T150405Z") + "-" + uuid.NewString()[:8]
	dir, err := config.SnapshotDir(p.Name, id)
	if err != nil {
		return model.SnapshotManifest{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		return model.SnapshotManifest{}, err
	}

	dumpPath := filepath.Join(dir, "dump.archive")
	logPath := filepath.Join(dir, "logs", "create.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return model.SnapshotManifest{}, err
	}
	defer logFile.Close()

	args := []string{"--format=custom", "--data-only", "--column-inserts", "--no-owner", "--no-privileges", "--file", dumpPath, "--dbname", db.ConnString(p, password)}
	for _, t := range opts.IncludeTables {
		args = append(args, "-t", t)
	}
	for _, t := range opts.ExcludeTables {
		args = append(args, "-T", t)
	}
	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		return model.SnapshotManifest{}, fmt.Errorf("pg_dump failed: %w", err)
	}

	checksum, err := fileChecksum(dumpPath)
	if err != nil {
		return model.SnapshotManifest{}, err
	}

	if err := config.SaveSchemaFingerprint(p.Name, id, sf); err != nil {
		return model.SnapshotManifest{}, err
	}

	dbVersion, _ := db.DBVersion(ctx, pool)
	m := model.SnapshotManifest{
		ID:                 id,
		Profile:            p.Name,
		CreatedAt:          time.Now().UTC(),
		DBVersion:          dbVersion,
		AppVersion:         s.AppVersion,
		DataArchive:        "dump.archive",
		SchemaFingerprint:  hash,
		SnapshotChecksum:   checksum,
		Tags:               opts.Tags,
		TableRowCounts:     counts,
		FilteredTables:     opts.IncludeTables,
		ExcludedTables:     opts.ExcludeTables,
		DeterministicOrder: opts.Deterministic,
		Metadata:           opts.Metadata,
	}
	if err := config.SaveManifest(p.Name, m); err != nil {
		return model.SnapshotManifest{}, err
	}
	_ = os.WriteFile(filepath.Join(dir, "checksums.sha256"), []byte(fmt.Sprintf("%s  %s\n", checksum, "dump.archive")), 0o644)
	_ = history.Append(model.AuditEvent{ID: uuid.NewString(), Type: "snapshot.create", Profile: p.Name, Snapshot: m.ID, CreatedAt: time.Now().UTC(), Status: "success"})
	return m, nil
}

func (s Service) PlanRestore(ctx context.Context, profile, snapshotID string) (RestorePlan, error) {
	p, sfSource, sfTarget, diffResult, err := s.preflight(ctx, profile, snapshotID)
	if err != nil {
		return RestorePlan{}, err
	}
	_ = p
	_ = sfSource

	requires := []model.ColumnSummary{}
	for tableKey, td := range diffResult.Tables {
		for _, c := range td.MissingInSource {
			if !c.IsNullable && c.Default == "" {
				requires = append(requires, model.ColumnSummary{
					Name:       tableKey + "." + c.Name,
					DataType:   c.DataType,
					UDTName:    c.UDTName,
					IsNullable: c.IsNullable,
					Default:    c.Default,
				})
			}
		}
	}
	sort.Slice(requires, func(i, j int) bool { return requires[i].Name < requires[j].Name })
	return RestorePlan{
		Compatible:     diffResult.Compatible,
		RequiresPrompt: requires,
		Diff:           diffResult,
		Warnings:       warningsFromDiff(sfTarget, diffResult),
	}, nil
}

func (s Service) Restore(ctx context.Context, opts model.RestoreOptions, prompt PromptFunc) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	p, sfSource, sfTarget, diffResult, err := s.preflight(ctx, opts.Profile, opts.SnapshotID)
	if err != nil {
		return err
	}
	polRes, err := policy.ValidateTarget(p.Host, cfg.Policy)
	if err != nil {
		return err
	}
	if polRes.RequiresWarningAck && !opts.ForceUnsafe {
		return errors.New("target requires warning acknowledgement; pass --force-unsafe to proceed")
	}

	if opts.DryRun {
		return nil
	}

	password, _ := config.LoadPassword(p.Name)
	pool, err := db.OpenPool(ctx, p, password)
	if err != nil {
		return err
	}
	defer pool.Close()

	if opts.TruncateBefore {
		if err := truncateAll(ctx, pool, sfTarget); err != nil {
			return err
		}
	}

	if diffResult.Compatible {
		if err := fastRestore(ctx, p, password, opts.SnapshotID); err != nil {
			_ = history.Append(model.AuditEvent{ID: uuid.NewString(), Type: "snapshot.restore", Profile: p.Name, Snapshot: opts.SnapshotID, CreatedAt: time.Now().UTC(), Status: "failed", Details: map[string]string{"reason": err.Error(), "mode": "fast"}})
			return err
		}
		_ = history.Append(model.AuditEvent{ID: uuid.NewString(), Type: "snapshot.restore", Profile: p.Name, Snapshot: opts.SnapshotID, CreatedAt: time.Now().UTC(), Status: "success", Details: map[string]string{"mode": "fast"}})
		return nil
	}

	rulePack, err := config.LoadRulePack(p.Name)
	if err != nil {
		return err
	}
	resolver := autofill.New(rulePack)

	insertSQL, err := extractInsertSQL(ctx, p, password, opts.SnapshotID)
	if err != nil {
		return err
	}

	targetTables := flattenTables(sfTarget)
	sourceTables := flattenTables(sfSource)
	promptCache := map[string]string{}
	var unresolved []model.ColumnSummary

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	scanner := bufio.NewScanner(strings.NewReader(insertSQL))
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 64*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		parsed, ok := parseInsertLine(line)
		if !ok {
			continue
		}
		tKey := parsed.Schema + "." + parsed.Table
		tgt, ok := targetTables[tKey]
		if !ok {
			continue
		}
		src, ok := sourceTables[tKey]
		if !ok {
			src = tgt
		}
		mapped, missing, err := mapInsert(parsed, src, tgt, resolver, opts.AutoFill, promptCache)
		if err != nil {
			return err
		}
		if len(missing) > 0 {
			unresolved = append(unresolved, missing...)
			continue
		}
		if mapped == "" {
			continue
		}
		if _, err := tx.Exec(ctx, mapped); err != nil {
			return fmt.Errorf("adaptive restore exec failed for %s: %w", tKey, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if len(unresolved) > 0 {
		names := make([]string, 0, len(unresolved))
		for _, c := range unresolved {
			names = append(names, c.Name)
		}
		sort.Strings(names)
		_ = prompt
		return fmt.Errorf("unresolved required fields: %s", strings.Join(names, ", "))
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = history.Append(model.AuditEvent{ID: uuid.NewString(), Type: "snapshot.restore", Profile: p.Name, Snapshot: opts.SnapshotID, CreatedAt: time.Now().UTC(), Status: "success", Details: map[string]string{"mode": "adaptive"}})
	return nil
}

func (s Service) preflight(ctx context.Context, profile, snapshotID string) (model.DBProfile, model.SchemaFingerprint, model.SchemaFingerprint, model.SchemaDiff, error) {
	p, err := config.LoadProfile(profile)
	if err != nil {
		return model.DBProfile{}, model.SchemaFingerprint{}, model.SchemaFingerprint{}, model.SchemaDiff{}, err
	}
	sfSource, err := config.LoadSchemaFingerprint(profile, snapshotID)
	if err != nil {
		return model.DBProfile{}, model.SchemaFingerprint{}, model.SchemaFingerprint{}, model.SchemaDiff{}, err
	}

	password, _ := config.LoadPassword(p.Name)
	pool, err := db.OpenPool(ctx, p, password)
	if err != nil {
		return model.DBProfile{}, model.SchemaFingerprint{}, model.SchemaFingerprint{}, model.SchemaDiff{}, err
	}
	defer pool.Close()

	sfTarget, _, err := db.CaptureSchemaFingerprint(ctx, pool)
	if err != nil {
		return model.DBProfile{}, model.SchemaFingerprint{}, model.SchemaFingerprint{}, model.SchemaDiff{}, err
	}

	diffResult := drift.Diff(sfSource, sfTarget)
	return p, sfSource, sfTarget, diffResult, nil
}

func fastRestore(ctx context.Context, p model.DBProfile, password, snapshotID string) error {
	dir, err := config.SnapshotDir(p.Name, snapshotID)
	if err != nil {
		return err
	}
	dumpPath := filepath.Join(dir, "dump.archive")
	logPath := filepath.Join(dir, "logs", "restore-fast.log")
	lf, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer lf.Close()

	args := []string{"--data-only", "--no-owner", "--no-privileges", "--dbname", db.ConnString(p, password), dumpPath}
	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Stdout = lf
	cmd.Stderr = lf
	return cmd.Run()
}

func extractInsertSQL(ctx context.Context, p model.DBProfile, password, snapshotID string) (string, error) {
	dir, err := config.SnapshotDir(p.Name, snapshotID)
	if err != nil {
		return "", err
	}
	dumpPath := filepath.Join(dir, "dump.archive")
	cmd := exec.CommandContext(ctx, "pg_restore", "--data-only", "--inserts", "--column-inserts", "-f", "-", dumpPath)
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("extract insert sql: %w", err)
	}
	return string(b), nil
}

type insertStmt struct {
	Schema string
	Table  string
	Cols   []string
	Vals   []string
}

func parseInsertLine(line string) (insertStmt, bool) {
	m := insertRegex.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) != 5 {
		return insertStmt{}, false
	}
	schema := strings.Trim(m[1], " .\"")
	if schema == "" {
		schema = "public"
	}
	table := strings.Trim(m[2], "\"")
	cols := splitCSV(m[3])
	vals := splitCSVValues(m[4])
	if len(cols) != len(vals) {
		return insertStmt{}, false
	}
	for i := range cols {
		cols[i] = strings.Trim(cols[i], "\" ")
	}
	return insertStmt{Schema: schema, Table: table, Cols: cols, Vals: vals}, true
}

func splitCSV(input string) []string {
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func splitCSVValues(input string) []string {
	out := []string{}
	cur := strings.Builder{}
	inQuote := false
	escape := false
	depth := 0
	for _, r := range input {
		switch {
		case escape:
			cur.WriteRune(r)
			escape = false
		case r == '\\':
			cur.WriteRune(r)
			escape = true
		case r == '\'':
			cur.WriteRune(r)
			inQuote = !inQuote
		case r == '(' && !inQuote:
			depth++
			cur.WriteRune(r)
		case r == ')' && !inQuote && depth > 0:
			depth--
			cur.WriteRune(r)
		case r == ',' && !inQuote && depth == 0:
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

func mapInsert(parsed insertStmt, source, target model.TableSummary, resolver autofill.Resolver, autoFill bool, promptCache map[string]string) (string, []model.ColumnSummary, error) {
	targetCols := map[string]model.ColumnSummary{}
	for _, c := range target.Columns {
		targetCols[c.Name] = c
	}

	srcMap := map[string]string{}
	for i := range parsed.Cols {
		srcMap[parsed.Cols[i]] = parsed.Vals[i]
	}

	outCols := []string{}
	outVals := []string{}
	missing := []model.ColumnSummary{}
	for _, tgtCol := range target.Columns {
		if v, ok := srcMap[tgtCol.Name]; ok {
			outCols = append(outCols, quoteIdent(tgtCol.Name))
			outVals = append(outVals, v)
			continue
		}
		if tgtCol.Default != "" {
			continue
		}
		if tgtCol.IsNullable {
			continue
		}
		key := parsed.Schema + "." + parsed.Table + "." + tgtCol.Name
		if cached, ok := promptCache[key]; ok {
			outCols = append(outCols, quoteIdent(tgtCol.Name))
			outVals = append(outVals, cached)
			continue
		}
		if v, ok := resolver.Resolve(tgtCol, parsed.Schema, parsed.Table); ok {
			outCols = append(outCols, quoteIdent(tgtCol.Name))
			outVals = append(outVals, v)
			continue
		}
		if autoFill {
			missing = append(missing, model.ColumnSummary{Name: key, DataType: tgtCol.DataType, UDTName: tgtCol.UDTName, IsNullable: tgtCol.IsNullable, Default: tgtCol.Default})
			continue
		}
		missing = append(missing, model.ColumnSummary{Name: key, DataType: tgtCol.DataType, UDTName: tgtCol.UDTName, IsNullable: tgtCol.IsNullable, Default: tgtCol.Default})
	}

	if len(outCols) == 0 {
		return "", missing, nil
	}
	stmt := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s);",
		quoteIdent(parsed.Schema), quoteIdent(parsed.Table),
		strings.Join(outCols, ", "), strings.Join(outVals, ", "))
	_ = source
	_ = targetCols
	return stmt, missing, nil
}

func flattenTables(sf model.SchemaFingerprint) map[string]model.TableSummary {
	out := map[string]model.TableSummary{}
	for _, s := range sf.Schemas {
		for _, t := range s.Tables {
			out[t.Schema+"."+t.Name] = t
		}
	}
	return out
}

func truncateAll(ctx context.Context, pool interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, sf model.SchemaFingerprint) error {
	tables := make([]string, 0)
	for _, s := range sf.Schemas {
		for _, t := range s.Tables {
			tables = append(tables, fmt.Sprintf("%s.%s", quoteIdent(t.Schema), quoteIdent(t.Name)))
		}
	}
	if len(tables) == 0 {
		return nil
	}
	_, err := pool.Exec(ctx, "TRUNCATE TABLE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE")
	return err
}

func warningsFromDiff(_ model.SchemaFingerprint, d model.SchemaDiff) []string {
	out := []string{}
	if d.Compatible {
		return out
	}
	for table, td := range d.Tables {
		if len(td.MissingInSource) > 0 {
			out = append(out, fmt.Sprintf("%s has %d new columns in target", table, len(td.MissingInSource)))
		}
		if len(td.TypeChanges) > 0 {
			out = append(out, fmt.Sprintf("%s has %d type changes", table, len(td.TypeChanges)))
		}
	}
	sort.Strings(out)
	return out
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode())
	})
}
