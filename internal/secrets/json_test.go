package secrets

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMapJSONStringsPreservesStructureAndPrecision(t *testing.T) {
	input := map[string]any{"url": "https://alice", "email": "password@example.test", "revision": uint64(9007199254740993), "nested": []any{true, nil, "https://user:pass@example.test", map[string]any{"sk-abcdefghijklmnopqrstuvwxyz": "value"}}}
	data, err := MapJSONStrings(input, NewRedactor().ScrubText)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]json.RawMessage
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if string(output["url"]) != `"https://alice"` || string(output["email"]) != `"password@example.test"` || string(output["revision"]) != "9007199254740993" {
		t.Fatalf("structure or precision changed: %s", data)
	}
	if strings.Contains(string(data), "user:pass") || strings.Contains(string(data), "sk-abcdefghijklmnopqrstuvwxyz") || !strings.Contains(string(data), "[REDACTED]") {
		t.Fatalf("nested credentials not scrubbed: %s", data)
	}
}

func TestMapJSONStringsRejectsKeyCollision(t *testing.T) {
	_, err := MapJSONStrings(map[string]string{"sk-abcdefghijklmnopqrstuvwxyz": "one", "[REDACTED]": "two"}, NewRedactor().ScrubText)
	if err == nil {
		t.Fatal("redaction silently overwrote a JSON field")
	}
	if strings.Contains(err.Error(), "sk-") {
		t.Fatal("collision error leaked key")
	}
}
