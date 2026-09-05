package rest

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func BuildUpdateQuery(schema, table string, updates map[string]any, filters url.Values) (Query, error) {
	if !validIdentifier(schema) || !validIdentifier(table) {
		return Query{}, errors.New("invalid table identifier")
	}
	if len(updates) == 0 {
		return Query{}, errors.New("update requires at least one column")
	}
	if len(filters) == 0 {
		return Query{}, errors.New("update requires a filter")
	}
	columns := make([]string, 0, len(updates))
	for column := range updates {
		if !validIdentifier(column) {
			return Query{}, errors.New("invalid update identifier")
		}
		columns = append(columns, column)
	}
	sort.Strings(columns)
	query := Query{SQL: fmt.Sprintf("UPDATE %s.%s SET ", quoteIdentifier(schema), quoteIdentifier(table))}
	assignments := make([]string, 0, len(columns))
	for _, column := range columns {
		query.Args = append(query.Args, updates[column])
		assignments = append(assignments, fmt.Sprintf("%s = $%d", quoteIdentifier(column), len(query.Args)))
	}
	query.SQL += strings.Join(assignments, ", ")
	where, args, err := buildWhere(filters, len(query.Args)+1)
	if err != nil {
		return Query{}, err
	}
	query.SQL += " WHERE " + where + " RETURNING *"
	query.Args = append(query.Args, args...)
	return query, nil
}

func BuildDeleteQuery(schema, table string, filters url.Values) (Query, error) {
	if !validIdentifier(schema) || !validIdentifier(table) {
		return Query{}, errors.New("invalid table identifier")
	}
	if len(filters) == 0 {
		return Query{}, errors.New("delete requires a filter")
	}
	where, args, err := buildWhere(filters, 1)
	if err != nil {
		return Query{}, err
	}
	return Query{SQL: fmt.Sprintf("DELETE FROM %s.%s WHERE %s RETURNING *", quoteIdentifier(schema), quoteIdentifier(table), where), Args: args}, nil
}

func buildWhere(values url.Values, argumentNumber int) (string, []any, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "select" && key != "order" && key != "limit" && key != "offset" && key != "on_conflict" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	conditions := make([]string, 0, len(keys))
	args := make([]any, 0)
	for _, key := range keys {
		if !validIdentifier(key) {
			return "", nil, errors.New("invalid filter identifier")
		}
		for _, expression := range values[key] {
			condition, conditionArgs, err := parseFilter(key, expression, argumentNumber+len(args))
			if err != nil {
				return "", nil, err
			}
			conditions = append(conditions, condition)
			args = append(args, conditionArgs...)
		}
	}
	if len(conditions) == 0 {
		return "", nil, errors.New("filter is required")
	}
	return strings.Join(conditions, " AND "), args, nil
}
