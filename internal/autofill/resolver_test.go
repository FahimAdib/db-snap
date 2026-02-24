package autofill

import (
	"testing"

	"db-snap/internal/model"
)

func TestResolverUsesExplicitRule(t *testing.T) {
	r := New(model.RulePack{Profile: "p", Rules: []model.ColumnRule{{Schema: "public", Table: "users", Column: "role", Value: "'admin'"}}})
	v, ok := r.Resolve(model.ColumnSummary{Name: "role", DataType: "text", UDTName: "text", IsNullable: false}, "public", "users")
	if !ok || v != "'admin'" {
		t.Fatalf("expected explicit rule, got ok=%v value=%s", ok, v)
	}
}

func TestResolverDefaultInt(t *testing.T) {
	r := New(model.RulePack{})
	v, ok := r.Resolve(model.ColumnSummary{Name: "age", DataType: "integer", UDTName: "int4", IsNullable: false}, "public", "users")
	if !ok || v != "0" {
		t.Fatalf("expected 0 for int, got ok=%v value=%s", ok, v)
	}
}
