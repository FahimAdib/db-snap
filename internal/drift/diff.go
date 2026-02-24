package drift

import (
	"fmt"

	"db-snap/internal/model"
)

func Diff(source, target model.SchemaFingerprint) model.SchemaDiff {
	result := model.SchemaDiff{Compatible: true, Tables: map[string]model.TableDiff{}}
	srcTables := flatten(source)
	tgtTables := flatten(target)

	for key, src := range srcTables {
		tgt, ok := tgtTables[key]
		td := model.TableDiff{TypeChanges: map[string]model.TypeChange{}}
		if !ok {
			result.Compatible = false
			td.MissingInTarget = append(td.MissingInTarget, "table_missing")
			result.Tables[key] = td
			continue
		}

		srcCols := colsByName(src.Columns)
		tgtCols := colsByName(tgt.Columns)

		for c, sCol := range srcCols {
			if tCol, ok := tgtCols[c]; ok {
				if sCol.UDTName != tCol.UDTName || sCol.DataType != tCol.DataType {
					td.TypeChanges[c] = model.TypeChange{From: fmt.Sprintf("%s(%s)", sCol.DataType, sCol.UDTName), To: fmt.Sprintf("%s(%s)", tCol.DataType, tCol.UDTName)}
					result.Compatible = false
				}
				continue
			}
			td.MissingInTarget = append(td.MissingInTarget, c)
			result.Compatible = false
		}

		for c, tCol := range tgtCols {
			if _, ok := srcCols[c]; !ok {
				td.MissingInSource = append(td.MissingInSource, tCol)
				if !tCol.IsNullable && tCol.Default == "" {
					result.Compatible = false
				}
			}
		}

		if len(td.MissingInTarget) > 0 || len(td.MissingInSource) > 0 || len(td.TypeChanges) > 0 {
			result.Tables[key] = td
		}
	}

	for key := range tgtTables {
		if _, ok := srcTables[key]; !ok {
			result.Tables[key] = model.TableDiff{MissingInSource: []model.ColumnSummary{{Name: "table_new"}}}
		}
	}

	return result
}

func flatten(sf model.SchemaFingerprint) map[string]model.TableSummary {
	out := map[string]model.TableSummary{}
	for _, s := range sf.Schemas {
		for _, t := range s.Tables {
			out[t.Schema+"."+t.Name] = t
		}
	}
	return out
}

func colsByName(cols []model.ColumnSummary) map[string]model.ColumnSummary {
	out := make(map[string]model.ColumnSummary, len(cols))
	for _, c := range cols {
		out[c.Name] = c
	}
	return out
}
