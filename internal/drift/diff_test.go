package drift

import (
	"testing"

	"db-snap/internal/model"
)

func TestDiffCompatible(t *testing.T) {
	s := sample("public", "users", []model.ColumnSummary{{Name: "id", DataType: "integer", UDTName: "int4", IsNullable: false}})
	d := Diff(s, s)
	if !d.Compatible {
		t.Fatalf("expected compatible")
	}
}

func TestDiffMissingRequiredColumnMakesIncompatible(t *testing.T) {
	src := sample("public", "users", []model.ColumnSummary{{Name: "id", DataType: "integer", UDTName: "int4", IsNullable: false}})
	tgt := sample("public", "users", []model.ColumnSummary{
		{Name: "id", DataType: "integer", UDTName: "int4", IsNullable: false},
		{Name: "email", DataType: "text", UDTName: "text", IsNullable: false},
	})
	d := Diff(src, tgt)
	if d.Compatible {
		t.Fatalf("expected incompatible")
	}
	if _, ok := d.Tables["public.users"]; !ok {
		t.Fatalf("expected table diff")
	}
}

func sample(schema, table string, cols []model.ColumnSummary) model.SchemaFingerprint {
	return model.SchemaFingerprint{Schemas: []model.SchemaSummary{{Name: schema, Tables: []model.TableSummary{{Schema: schema, Name: table, Columns: cols}}}}}
}
