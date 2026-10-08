package secrets

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScrubTextRemovesCredentialCanaries(t *testing.T) {
	redactor := NewRedactor()
	canaries := []string{
		"sk-test-HARNESSSCOPE-CANARY-1234567890",
		"ghp_HARNESSSCOPE_CANARY_1234567890",
		"Bearer HARNESSSCOPE-CANARY-1234567890",
		"-----BEGIN PRIVATE KEY-----\nHARNESSSCOPE-CANARY\n-----END PRIVATE KEY-----",
	}

	for _, raw := range canaries {
		got := redactor.ScrubText("error value=" + raw)
		if strings.Contains(got, raw) || !strings.Contains(got, "[REDACTED]") {
			t.Fatalf("secret %q leaked in %q", raw, got)
		}
	}
}

func TestRedactFieldUsesCredentialFieldName(t *testing.T) {
	got := NewRedactor().RedactField("mcp.headers.authorization", "ordinary-looking-value")

	if got.Display != "[REDACTED]" || got.SecretCategory != "credential_field" || !got.Present {
		t.Fatalf("unexpected safe value: %#v", got)
	}
}

func TestRedactTreeCannotSerializeNestedSecret(t *testing.T) {
	raw := map[string]any{
		"server": map[string]any{
			"api_key": "sk-test-HARNESSSCOPE-CANARY-1234567890",
			"command": "/usr/local/bin/server",
		},
	}

	safe := NewRedactor().RedactTree(raw, "")
	encoded, err := json.Marshal(safe)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "HARNESSSCOPE-CANARY") {
		t.Fatalf("nested secret leaked: %s", encoded)
	}
	if !strings.Contains(string(encoded), "/usr/local/bin/server") {
		t.Fatalf("non-secret path was lost: %s", encoded)
	}
}

func TestOrdinaryPathIsNotHighEntropySecret(t *testing.T) {
	got := NewRedactor().RedactField("mcp.command", "/Users/example/projects/harnessscope/bin/server")

	if got.Display == "[REDACTED]" {
		t.Fatalf("ordinary path was redacted: %#v", got)
	}
}
