package autofill

import (
	"fmt"
	"strings"
	"time"

	"db-snap/internal/model"
	"github.com/google/uuid"
)

type Resolver struct {
	rules map[string]model.ColumnRule
}

func New(rulePack model.RulePack) Resolver {
	m := make(map[string]model.ColumnRule, len(rulePack.Rules))
	for _, r := range rulePack.Rules {
		m[key(r.Schema, r.Table, r.Column)] = r
	}
	return Resolver{rules: m}
}

func (r Resolver) Resolve(col model.ColumnSummary, schema, table string) (string, bool) {
	if rule, ok := r.rules[key(schema, table, col.Name)]; ok {
		return rule.Value, true
	}
	udt := strings.ToLower(col.UDTName)
	dataType := strings.ToLower(col.DataType)

	switch {
	case strings.Contains(udt, "uuid"):
		return quote(uuid.NewString()), true
	case dataType == "timestamp with time zone" || dataType == "timestamp without time zone":
		return quote(time.Now().UTC().Format(time.RFC3339Nano)), true
	case dataType == "date":
		return quote(time.Now().UTC().Format("2006-01-02")), true
	case dataType == "boolean":
		return "false", true
	case strings.Contains(dataType, "character") || strings.Contains(dataType, "text"):
		return quote(""), true
	case strings.Contains(dataType, "int") || strings.Contains(dataType, "numeric") || strings.Contains(dataType, "double") || strings.Contains(dataType, "real"):
		return "0", true
	case strings.Contains(dataType, "json"):
		return quote("{}"), true
	case strings.Contains(dataType, "array"):
		return "'{}'", true
	default:
		return "", false
	}
}

func quote(v string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
}

func key(schema, table, col string) string {
	return schema + "." + table + "." + col
}
