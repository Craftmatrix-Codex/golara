package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type Query struct {
	SQL  string
	Args []any
}

func BuildSelectQuery(schema, table string, values url.Values) (Query, error) {
	if !validIdentifier(schema) || !validIdentifier(table) {
		return Query{}, errors.New("invalid table identifier")
	}
	columns, err := selectColumns(values.Get("select"))
	if err != nil {
		return Query{}, err
	}
	query := Query{SQL: fmt.Sprintf("SELECT %s FROM %s.%s", columns, quoteIdentifier(schema), quoteIdentifier(table))}
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "select" && key != "order" && key != "limit" && key != "offset" && key != "on_conflict" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	conditions := make([]string, 0, len(keys))
	for _, key := range keys {
		if !validIdentifier(key) {
			return Query{}, errors.New("invalid filter identifier")
		}
		for _, expression := range values[key] {
			condition, args, err := parseFilter(key, expression, len(query.Args)+1)
			if err != nil {
				return Query{}, err
			}
			conditions = append(conditions, condition)
			query.Args = append(query.Args, args...)
		}
	}
	if len(conditions) > 0 {
		query.SQL += " WHERE " + strings.Join(conditions, " AND ")
	}
	if order := values.Get("order"); order != "" {
		orderSQL, err := parseOrder(order)
		if err != nil {
			return Query{}, err
		}
		query.SQL += " ORDER BY " + orderSQL
	}
	for _, key := range []string{"limit", "offset"} {
		if raw := values.Get(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 || value > 1_000_000 {
				return Query{}, fmt.Errorf("invalid %s", key)
			}
			query.SQL += fmt.Sprintf(" %s %d", strings.ToUpper(key), value)
		}
	}
	return query, nil
}

func BuildInsertQuery(schema, table string, rows []map[string]any) (Query, error) {
	if !validIdentifier(schema) || !validIdentifier(table) {
		return Query{}, errors.New("invalid table identifier")
	}
	if len(rows) == 0 {
		return Query{}, errors.New("insert requires at least one row")
	}
	columns := make([]string, 0, len(rows[0]))
	for column := range rows[0] {
		if !validIdentifier(column) {
			return Query{}, errors.New("invalid insert identifier")
		}
		columns = append(columns, column)
	}
	if len(columns) == 0 {
		return Query{}, errors.New("insert requires at least one column")
	}
	sort.Strings(columns)
	quotedColumns := make([]string, len(columns))
	for index, column := range columns {
		quotedColumns[index] = quoteIdentifier(column)
	}
	query := Query{SQL: fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES ", quoteIdentifier(schema), quoteIdentifier(table), strings.Join(quotedColumns, ", "))}
	valueGroups := make([]string, 0, len(rows))
	for rowIndex, row := range rows {
		placeholders := make([]string, len(columns))
		for columnIndex, column := range columns {
			value, ok := row[column]
			if !ok {
				return Query{}, fmt.Errorf("row %d is missing column %q", rowIndex, column)
			}
			argument, err := normalizeInsertValue(value)
			if err != nil {
				return Query{}, err
			}
			query.Args = append(query.Args, argument)
			placeholders[columnIndex] = fmt.Sprintf("$%d", len(query.Args))
		}
		valueGroups = append(valueGroups, "("+strings.Join(placeholders, ", ")+")")
	}
	query.SQL += strings.Join(valueGroups, ", ") + " RETURNING *"
	return query, nil
}

func normalizeInsertValue(value any) (any, error) {
	switch value.(type) {
	case map[string]any, []any:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode insert value: %w", err)
		}
		return encoded, nil
	default:
		return value, nil
	}
}

func selectColumns(raw string) (string, error) {
	if raw == "" || raw == "*" {
		return "*", nil
	}
	parts := strings.Split(raw, ",")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		if !validIdentifier(part) {
			return "", errors.New("invalid select identifier")
		}
		quoted = append(quoted, quoteIdentifier(part))
	}
	return strings.Join(quoted, ", "), nil
}

func parseFilter(column, expression string, argumentNumber int) (string, []any, error) {
	separator := strings.IndexByte(expression, '.')
	if separator <= 0 || separator == len(expression)-1 {
		return "", nil, errors.New("invalid filter")
	}
	operator, value := expression[:separator], expression[separator+1:]
	columnSQL := quoteIdentifier(column)
	switch operator {
	case "eq":
		return fmt.Sprintf("%s = $%d", columnSQL, argumentNumber), []any{value}, nil
	case "neq":
		return fmt.Sprintf("%s <> $%d", columnSQL, argumentNumber), []any{value}, nil
	case "gt":
		return fmt.Sprintf("%s > $%d", columnSQL, argumentNumber), []any{value}, nil
	case "gte":
		return fmt.Sprintf("%s >= $%d", columnSQL, argumentNumber), []any{value}, nil
	case "lt":
		return fmt.Sprintf("%s < $%d", columnSQL, argumentNumber), []any{value}, nil
	case "lte":
		return fmt.Sprintf("%s <= $%d", columnSQL, argumentNumber), []any{value}, nil
	case "like":
		return fmt.Sprintf("%s LIKE $%d", columnSQL, argumentNumber), []any{value}, nil
	case "ilike":
		return fmt.Sprintf("%s ILIKE $%d", columnSQL, argumentNumber), []any{value}, nil
	case "is":
		switch strings.ToLower(value) {
		case "null":
			return columnSQL + " IS NULL", nil, nil
		case "true":
			return columnSQL + " IS TRUE", nil, nil
		case "false":
			return columnSQL + " IS FALSE", nil, nil
		default:
			return "", nil, errors.New("invalid is filter")
		}
	default:
		return "", nil, fmt.Errorf("unsupported filter operator %q", operator)
	}
}

func parseOrder(raw string) (string, error) {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		fields := strings.Split(part, ".")
		if len(fields) < 1 || len(fields) > 2 || !validIdentifier(fields[0]) {
			return "", errors.New("invalid order")
		}
		direction := "ASC"
		if len(fields) == 2 {
			switch strings.ToLower(fields[1]) {
			case "asc":
				direction = "ASC"
			case "desc":
				direction = "DESC"
			default:
				return "", errors.New("invalid order direction")
			}
		}
		result = append(result, quoteIdentifier(fields[0])+" "+direction)
	}
	return strings.Join(result, ", "), nil
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		digit := character >= '0' && character <= '9'
		if !letter && !digit && character != '_' {
			return false
		}
		if index == 0 && digit {
			return false
		}
	}
	return true
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
