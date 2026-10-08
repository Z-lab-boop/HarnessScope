package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRootHelpListsStableCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), []string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, name := range []string{"scan", "explain", "compare", "report", "fix", "rollback"} {
		if !strings.Contains(stdout.String(), name) {
			t.Errorf("help is missing %q", name)
		}
	}
}
