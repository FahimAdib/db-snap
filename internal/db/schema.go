package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"db-snap/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

func DBVersion(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var v string
	if err := pool.QueryRow(ctx, `select version()`).Scan(&v); err != nil {
		return "", err
	}
	return v, nil
}

func CaptureSchemaFingerprint(ctx context.Context, pool *pgxpool.Pool) (model.SchemaFingerprint, string, error) {
	v, err := DBVersion(ctx, pool)
	if err != nil {
		return model.SchemaFingerprint{}, "", err
	}

	rows, err := pool.Query(ctx, `
select c.table_schema, c.table_name, c.column_name, c.data_type, c.udt_name,
       c.is_nullable = 'YES' as is_nullable,
       coalesce(c.column_default, '') as column_default
from information_schema.columns c
join information_schema.tables t
  on t.table_schema = c.table_schema and t.table_name = c.table_name
where t.table_type='BASE TABLE'
  and c.table_schema not in ('pg_catalog','information_schema')
order by c.table_schema, c.table_name, c.ordinal_position;
`)
	if err != nil {
		return model.SchemaFingerprint{}, "", err
	}
	defer rows.Close()

	tableMap := map[string]*model.TableSummary{}
	for rows.Next() {
		var schema, table, column, dataType, udt, def string
		var nullable bool
		if err := rows.Scan(&schema, &table, &column, &dataType, &udt, &nullable, &def); err != nil {
			return model.SchemaFingerprint{}, "", err
		}
		key := schema + "." + table
		t, ok := tableMap[key]
		if !ok {
			t = &model.TableSummary{Schema: schema, Name: table}
			tableMap[key] = t
		}
		t.Columns = append(t.Columns, model.ColumnSummary{
			Name:       column,
			DataType:   dataType,
			UDTName:    udt,
			IsNullable: nullable,
			Default:    def,
		})
	}
	if err := rows.Err(); err != nil {
		return model.SchemaFingerprint{}, "", err
	}

	pkRows, err := pool.Query(ctx, `
select tc.table_schema, tc.table_name, kcu.column_name
from information_schema.table_constraints tc
join information_schema.key_column_usage kcu
  on tc.constraint_name = kcu.constraint_name
 and tc.table_schema = kcu.table_schema
where tc.constraint_type = 'PRIMARY KEY'
order by tc.table_schema, tc.table_name, kcu.ordinal_position;
`)
	if err != nil {
		return model.SchemaFingerprint{}, "", err
	}
	defer pkRows.Close()
	for pkRows.Next() {
		var s, t, c string
		if err := pkRows.Scan(&s, &t, &c); err != nil {
			return model.SchemaFingerprint{}, "", err
		}
		key := s + "." + t
		if tbl, ok := tableMap[key]; ok {
			tbl.PrimaryKeys = append(tbl.PrimaryKeys, c)
		}
	}

	fkRows, err := pool.Query(ctx, `
select tc.table_schema, tc.table_name, kcu.column_name,
       ccu.table_schema as foreign_table_schema,
       ccu.table_name as foreign_table_name,
       ccu.column_name as foreign_column_name,
       tc.constraint_name
from information_schema.table_constraints tc
join information_schema.key_column_usage kcu
  on tc.constraint_name = kcu.constraint_name and tc.table_schema = kcu.table_schema
join information_schema.constraint_column_usage ccu
  on ccu.constraint_name = tc.constraint_name and ccu.constraint_schema = tc.table_schema
where tc.constraint_type = 'FOREIGN KEY'
order by tc.table_schema, tc.table_name, tc.constraint_name, kcu.ordinal_position;
`)
	if err != nil {
		return model.SchemaFingerprint{}, "", err
	}
	defer fkRows.Close()
	type fkKey struct {
		table string
		name  string
	}
	fkAgg := map[fkKey]*model.ForeignKey{}
	for fkRows.Next() {
		var s, t, c, rs, rt, rc, cname string
		if err := fkRows.Scan(&s, &t, &c, &rs, &rt, &rc, &cname); err != nil {
			return model.SchemaFingerprint{}, "", err
		}
		k := fkKey{table: s + "." + t, name: cname}
		if _, ok := fkAgg[k]; !ok {
			fkAgg[k] = &model.ForeignKey{RefSchema: rs, RefTable: rt}
		}
		fkAgg[k].Columns = append(fkAgg[k].Columns, c)
		fkAgg[k].RefColumns = append(fkAgg[k].RefColumns, rc)
	}
	for k, fk := range fkAgg {
		if tbl, ok := tableMap[k.table]; ok {
			tbl.ForeignKeys = append(tbl.ForeignKeys, *fk)
		}
	}

	tables := make([]model.TableSummary, 0, len(tableMap))
	for _, t := range tableMap {
		sort.Slice(t.Columns, func(i, j int) bool { return t.Columns[i].Name < t.Columns[j].Name })
		tables = append(tables, *t)
	}
	sort.Slice(tables, func(i, j int) bool {
		if tables[i].Schema == tables[j].Schema {
			return tables[i].Name < tables[j].Name
		}
		return tables[i].Schema < tables[j].Schema
	})

	schemaGroups := map[string][]model.TableSummary{}
	for _, t := range tables {
		schemaGroups[t.Schema] = append(schemaGroups[t.Schema], t)
	}
	schemas := make([]model.SchemaSummary, 0, len(schemaGroups))
	for s, ts := range schemaGroups {
		schemas = append(schemas, model.SchemaSummary{Name: s, Tables: ts})
	}
	sort.Slice(schemas, func(i, j int) bool { return schemas[i].Name < schemas[j].Name })

	sf := model.SchemaFingerprint{GeneratedAt: time.Now().UTC(), DBVersion: v, Schemas: schemas}
	b, err := json.Marshal(sf)
	if err != nil {
		return model.SchemaFingerprint{}, "", err
	}
	h := sha256.Sum256(b)
	return sf, hex.EncodeToString(h[:]), nil
}

func TableRowCounts(ctx context.Context, pool *pgxpool.Pool) (map[string]int64, error) {
	rows, err := pool.Query(ctx, `
select quote_ident(schemaname)||'.'||quote_ident(relname) as table_name,
       n_live_tup::bigint
from pg_stat_user_tables
order by 1;
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var name string
		var c int64
		if err := rows.Scan(&name, &c); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(name)] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row count scan: %w", err)
	}
	return out, nil
}
