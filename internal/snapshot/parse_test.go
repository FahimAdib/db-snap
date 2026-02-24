package snapshot

import "testing"

func TestParseInsertLine(t *testing.T) {
	line := `INSERT INTO public.users (id, name, meta) VALUES (1, 'Jane, Doe', '{"k":"v"}');`
	stmt, ok := parseInsertLine(line)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if stmt.Schema != "public" || stmt.Table != "users" {
		t.Fatalf("unexpected target: %s.%s", stmt.Schema, stmt.Table)
	}
	if len(stmt.Cols) != 3 || len(stmt.Vals) != 3 {
		t.Fatalf("unexpected columns/values count")
	}
	if stmt.Vals[1] != "'Jane, Doe'" {
		t.Fatalf("expected quoted comma value, got %s", stmt.Vals[1])
	}
}
