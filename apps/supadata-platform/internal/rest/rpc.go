package rest

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

func BuildRPCQuery(schema, function string, arguments map[string]any) (Query, error) {
	if !validIdentifier(schema) || !validIdentifier(function) {
		return Query{}, errors.New("invalid function identifier")
	}
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		if !validIdentifier(key) {
			return Query{}, errors.New("invalid function argument identifier")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	args := make([]any, len(keys))
	for index, key := range keys {
		parts[index] = fmt.Sprintf(`%s := $%d`, quoteIdentifier(key), index+1)
		args[index] = arguments[key]
	}
	return Query{
		SQL:  fmt.Sprintf(`SELECT * FROM %s.%s(%s)`, quoteIdentifier(schema), quoteIdentifier(function), strings.Join(parts, ", ")),
		Args: args,
	}, nil
}
