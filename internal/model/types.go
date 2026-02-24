package model

import "time"

type AppConfig struct {
	Version string       `yaml:"version"`
	Policy  SafetyPolicy `yaml:"policy"`
}

type SafetyPolicy struct {
	AllowCIDRs       []string `yaml:"allowCidrs"`
	DenyHostKeywords []string `yaml:"denyHostKeywords"`
	WarnEveryRestore bool     `yaml:"warnEveryRestore"`
}

type DBProfile struct {
	Name      string   `yaml:"name" json:"name"`
	Host      string   `yaml:"host" json:"host"`
	Port      int      `yaml:"port" json:"port"`
	Database  string   `yaml:"database" json:"database"`
	User      string   `yaml:"user" json:"user"`
	SSLMode   string   `yaml:"sslMode" json:"sslMode"`
	Tags      []string `yaml:"tags" json:"tags"`
	CreatedAt string   `yaml:"createdAt" json:"createdAt"`
	UpdatedAt string   `yaml:"updatedAt" json:"updatedAt"`
}

type SnapshotManifest struct {
	ID                 string            `json:"id"`
	Profile            string            `json:"profile"`
	CreatedAt          time.Time         `json:"createdAt"`
	DBVersion          string            `json:"dbVersion"`
	AppVersion         string            `json:"appVersion"`
	DataArchive        string            `json:"dataArchive"`
	SchemaFingerprint  string            `json:"schemaFingerprint"`
	SnapshotChecksum   string            `json:"snapshotChecksum"`
	Tags               []string          `json:"tags"`
	TableRowCounts     map[string]int64  `json:"tableRowCounts"`
	FilteredTables     []string          `json:"filteredTables,omitempty"`
	ExcludedTables     []string          `json:"excludedTables,omitempty"`
	DeterministicOrder bool              `json:"deterministicOrder"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type SchemaFingerprint struct {
	GeneratedAt time.Time       `json:"generatedAt"`
	DBVersion   string          `json:"dbVersion"`
	Schemas     []SchemaSummary `json:"schemas"`
}

type SchemaSummary struct {
	Name   string         `json:"name"`
	Tables []TableSummary `json:"tables"`
}

type TableSummary struct {
	Schema      string          `json:"schema"`
	Name        string          `json:"name"`
	Columns     []ColumnSummary `json:"columns"`
	PrimaryKeys []string        `json:"primaryKeys"`
	Uniques     [][]string      `json:"uniques,omitempty"`
	Checks      []string        `json:"checks,omitempty"`
	ForeignKeys []ForeignKey    `json:"foreignKeys,omitempty"`
}

type ColumnSummary struct {
	Name       string `json:"name"`
	DataType   string `json:"dataType"`
	UDTName    string `json:"udtName"`
	IsNullable bool   `json:"isNullable"`
	Default    string `json:"default,omitempty"`
}

type ForeignKey struct {
	Columns    []string `json:"columns"`
	RefSchema  string   `json:"refSchema"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
}

type SchemaDiff struct {
	Compatible bool                 `json:"compatible"`
	Tables     map[string]TableDiff `json:"tables"`
}

type TableDiff struct {
	MissingInTarget []string              `json:"missingInTarget,omitempty"`
	MissingInSource []ColumnSummary       `json:"missingInSource,omitempty"`
	TypeChanges     map[string]TypeChange `json:"typeChanges,omitempty"`
}

type TypeChange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ColumnRule struct {
	Schema     string `yaml:"schema" json:"schema"`
	Table      string `yaml:"table" json:"table"`
	Column     string `yaml:"column" json:"column"`
	Type       string `yaml:"type" json:"type"`
	Value      string `yaml:"value" json:"value"`
	Persistent bool   `yaml:"persistent" json:"persistent"`
}

type RulePack struct {
	Profile string       `yaml:"profile" json:"profile"`
	Rules   []ColumnRule `yaml:"rules" json:"rules"`
}

type RestoreOptions struct {
	SnapshotID     string
	Profile        string
	AutoFill       bool
	DryRun         bool
	ForceUnsafe    bool
	TruncateBefore bool
}

type AuditEvent struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Profile   string            `json:"profile"`
	Snapshot  string            `json:"snapshot,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	Status    string            `json:"status"`
	Details   map[string]string `json:"details,omitempty"`
}
