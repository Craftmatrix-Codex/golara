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
		if key != "or" && !validIdentifier(key) {
			return Query{}, errors.New("invalid filter identifier")
		}
		for _, expression := range values[key] {
			condition, args, err := buildFilter(key, expression, len(query.Args)+1)
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

func buildFilter(key, expression string, argumentNumber int) (string, []any, error) {
	if key == "or" {
		return parseOrFilter(expression, argumentNumber)
	}
	return parseFilter(key, expression, argumentNumber)
}

func parseOrFilter(raw string, argumentNumber int) (string, []any, error) {
	if len(raw) < 2 || raw[0] != '(' || raw[len(raw)-1] != ')' {
		return "", nil, errors.New("invalid or filter")
	}
	body := raw[1 : len(raw)-1]
	parts := make([]string, 0, 2)
	start, depth := 0, 0
	quoted, escaped := false, false
	for index, character := range body {
		if escaped {
			escaped = false
			continue
		}
		if quoted && character == '\\' {
			escaped = true
			continue
		}
		if character == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		switch character {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:index])
				start = index + 1
			}
		}
		if depth < 0 {
			return "", nil, errors.New("invalid or filter")
		}
	}
	if quoted || escaped || depth != 0 {
		return "", nil, errors.New("invalid or filter")
	}
	parts = append(parts, body[start:])
	if len(parts) < 2 {
		return "", nil, errors.New("or filter requires two conditions")
	}
	conditions := make([]string, 0, len(parts))
	args := make([]any, 0)
	for _, part := range parts {
		separator := strings.IndexByte(part, '.')
		if separator <= 0 || separator == len(part)-1 || !validIdentifier(part[:separator]) {
			return "", nil, errors.New("invalid or condition")
		}
		condition, conditionArgs, err := parseFilter(part[:separator], part[separator+1:], argumentNumber+len(args))
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args, nil
}

func parseFilter(column, expression string, argumentNumber int) (string, []any, error) {
	if strings.HasPrefix(expression, "not.") {
		condition, args, err := parseFilter(column, strings.TrimPrefix(expression, "not."), argumentNumber)
		if err != nil {
			return "", nil, err
		}
		return "NOT (" + condition + ")", args, nil
	}
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
	case "in":
		values, err := parseInValues(value)
		if err != nil {
			return "", nil, err
		}
		placeholders := make([]string, len(values))
		for index := range values {
			placeholders[index] = fmt.Sprintf("$%d", argumentNumber+index)
		}
		arguments := make([]any, len(values))
		for index, item := range values {
			arguments[index] = item
		}
		return fmt.Sprintf("%s IN (%s)", columnSQL, strings.Join(placeholders, ", ")), arguments, nil
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

func parseInValues(raw string) ([]string, error) {
	if len(raw) < 2 || raw[0] != '(' || raw[len(raw)-1] != ')' {
		return nil, errors.New("invalid in filter")
	}
	body := raw[1 : len(raw)-1]
	if body == "" {
		return nil, errors.New("in filter requires values")
	}
	parts := make([]string, 0, 4)
	start := 0
	quoted := false
	escaped := false
	for index, character := range body {
		if escaped {
			escaped = false
			continue
		}
		if quoted && character == '\\' {
			escaped = true
			continue
		}
		if character == '"' {
			quoted = !quoted
			continue
		}
		if character == ',' && !quoted {
			parts = append(parts, body[start:index])
			start = index + 1
		}
	}
	if quoted || escaped {
		return nil, errors.New("invalid quoted in filter")
	}
	parts = append(parts, body[start:])
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("in filter contains an empty value")
		}
		if strings.HasPrefix(part, "\"") || strings.HasSuffix(part, "\"") {
			decoded, err := strconv.Unquote(part)
			if err != nil {
				return nil, errors.New("invalid quoted in filter")
			}
			part = decoded
		}
		parts[index] = part
	}
	return parts, nil
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
