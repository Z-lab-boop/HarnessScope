package secrets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// MapJSONStrings transforms individual decoded strings and object keys before
// serializing. Numbers retain their original precision. A key collision fails
// closed instead of silently dropping a field after redaction.
func MapJSONStrings(value any, transform func(string) string) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	var visit func(any) (any, error)
	visit = func(value any) (any, error) {
		switch typed := value.(type) {
		case string:
			return transform(typed), nil
		case []any:
			for i := range typed {
				child, err := visit(typed[i])
				if err != nil {
					return nil, err
				}
				typed[i] = child
			}
		case map[string]any:
			mapped := make(map[string]any, len(typed))
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				clean := transform(key)
				if _, exists := mapped[clean]; exists {
					return nil, fmt.Errorf("JSON field collision after redaction")
				}
				child, err := visit(typed[key])
				if err != nil {
					return nil, err
				}
				mapped[clean] = child
			}
			return mapped, nil
		}
		return value, nil
	}
	safe, err := visit(decoded)
	if err != nil {
		return nil, err
	}
	return json.Marshal(safe)
}
