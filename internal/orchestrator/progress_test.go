package orchestrator

import (
	"bytes"
	"strings"
	"testing"
)

func TestProgressTrackerShowsStepPercentAndCompletion(t *testing.T) {
	var out bytes.Buffer
	p := newProgressTracker(&out, "sentinel", 2)
	p.start("A00", "Bootstrap")
	p.complete("A00", "Bootstrap")
	p.start("A10", "Overview")
	p.complete("A10", "Overview")
	text := out.String()
	for _, expected := range []string{"step 1/2", " 50%", "100%", "A00", "A10", "sentinel"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in progress output:\n%s", expected, text)
		}
	}
}
