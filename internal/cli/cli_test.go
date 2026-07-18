package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpListsCurrentCommands(t *testing.T) {
	var out bytes.Buffer
	code := (CLI{Out: &out, Err: &out}).Run(context.Background(), []string{"help"})
	if code != 0 {
		t.Fatalf("help exit code=%d", code)
	}
	for _, command := range []string{"discover", "plan", "generate", "update", "validate", "coverage", "impact", "graph"} {
		if !strings.Contains(out.String(), command) {
			t.Fatalf("help does not list %q:\n%s", command, out.String())
		}
	}
	for _, removed := range []string{"legacy-layout", "legacy-phases", "config migrate", "docs migrate-layout", "--service"} {
		if strings.Contains(out.String(), removed) {
			t.Fatalf("help still exposes removed legacy surface %q:\n%s", removed, out.String())
		}
	}
}
