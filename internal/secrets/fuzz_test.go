package secrets

import (
	"strings"
	"testing"
)

func FuzzRedactorNeverReturnsKnownCredentialLiteral(f *testing.F) {
	for _, seed := range []string{
		"sk-test-HARNESSSCOPE-CANARY-1234567890",
		"ghp_HARNESSSCOPE_CANARY_1234567890",
		"Bearer HARNESSSCOPE-CANARY-1234567890",
		"https://user:password@example.test/path",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		redactor := NewRedactor()
		output := redactor.ScrubText(value)
		for _, marker := range []string{"sk-test-HARNESSSCOPE-CANARY-1234567890", "ghp_HARNESSSCOPE_CANARY_1234567890", "Bearer HARNESSSCOPE-CANARY-1234567890", "user:password@"} {
			if strings.Contains(value, marker) && strings.Contains(output, marker) {
				t.Fatalf("credential marker propagated: %q", output)
			}
		}
		field := redactor.RedactField("headers.authorization", value)
		if field.Display != "[REDACTED]" {
			t.Fatalf("credential field was not redacted: %#v", field)
		}
	})
}
