package rest

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

func BuildUpsertQuery(schema, table string, rows []map[string]any, rawConflict string) (Query, error) {
	if strings.TrimSpace(rawConflict) == "" {
		return Query{}, errors.New("upsert requires on_conflict")
	}
	query, err := BuildInsertQuery(schema, table, rows)
	if err != nil {
		return Query{}, err
	}
	columns := make([]string, 0, len(rows[0]))
	for column := range rows[0] {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	available := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		available[column] = struct{}{}
	}
	conflictColumns := strings.Split(rawConflict, ",")
	quotedConflict := make([]string, 0, len(conflictColumns))
	conflictSet := make(map[string]struct{}, len(conflictColumns))
	for _, column := range conflictColumns {
		column = strings.TrimSpace(column)
		if !validIdentifier(column) {
			return Query{}, errors.New("invalid conflict identifier")
		}
		if _, ok := available[column]; !ok {
			return Query{}, fmt.Errorf("conflict column %q is not present in insert", column)
		}
		if _, duplicate := conflictSet[column]; duplicate {
			return Query{}, errors.New("duplicate conflict identifier")
		}
		conflictSet[column] = struct{}{}
		quotedConflict = append(quotedConflict, quoteIdentifier(column))
	}
	base := strings.TrimSuffix(query.SQL, " RETURNING *")
	updates := make([]string, 0, len(columns))
	for _, column := range columns {
		if _, isConflict := conflictSet[column]; isConflict {
			continue
		}
		quoted := quoteIdentifier(column)
		updates = append(updates, quoted+" = EXCLUDED."+quoted)
	}
	if len(updates) == 0 {
		base += " ON CONFLICT (" + strings.Join(quotedConflict, ", ") + ") DO NOTHING"
	} else {
		base += " ON CONFLICT (" + strings.Join(quotedConflict, ", ") + ") DO UPDATE SET " + strings.Join(updates, ", ")
	}
	query.SQL = base + " RETURNING *"
	return query, nil
}
