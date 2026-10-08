package formats

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONCPreservesCommentMarkersInsideStrings(t *testing.T) {
	input := []byte(`{
  // real comment
  "url": "https://example.test/a//b",
  "pattern": "/* literal */",
  "items": [1, 2,],
}`)
	cleaned, err := NormalizeJSONC(input)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(cleaned, &decoded); err != nil {
		t.Fatalf("normalized JSON is invalid: %v\n%s", err, cleaned)
	}
	if decoded["url"] != "https://example.test/a//b" || decoded["pattern"] != "/* literal */" {
		t.Fatalf("string content changed: %#v", decoded)
	}
}

func TestJSONCRejectsUnterminatedBlockComment(t *testing.T) {
	_, err := NormalizeJSONC([]byte(`{"ok": true /* never closed`))
	if err == nil || !strings.Contains(err.Error(), "unterminated block comment") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJSONCRejectsUnterminatedString(t *testing.T) {
	_, err := NormalizeJSONC([]byte(`{"value": "never closed}`))
	if err == nil || !strings.Contains(err.Error(), "unterminated string") {
		t.Fatalf("unexpected error: %v", err)
	}
}
