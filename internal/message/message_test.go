package message

import (
	"strings"
	"testing"

	"github.com/aghaie/job-finder/internal/jsearch"
	"github.com/aghaie/job-finder/internal/summarize"
)

func TestBuild(t *testing.T) {
	j := jsearch.Job{
		Title:       "Backend Developer",
		Company:     "Acme GmbH",
		City:        "Berlin",
		Country:     "DE",
		IsRemote:    true,
		Description: "We build backend systems in Go.",
		ApplyLink:   "https://example.com/apply",
		PostedAt:    "2026-07-24T10:00:00.000Z",
	}
	sum := summarize.NewSimple(200)
	out := Build(j, sum)

	for _, want := range []string{
		"Backend Developer",
		"Acme GmbH",
		"Berlin",
		"https://example.com/apply",
		"2026-07-24",
		"Remote",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("message missing %q\n---\n%s", want, out)
		}
	}
}

func TestBuildHandlesMissingLocation(t *testing.T) {
	j := jsearch.Job{
		Title:     "Dev",
		Company:   "X",
		ApplyLink: "https://x.test",
	}
	out := Build(j, summarize.NewSimple(200))
	if !strings.Contains(out, "Dev") || !strings.Contains(out, "https://x.test") {
		t.Errorf("unexpected: %s", out)
	}
}
