package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/aghaie/job-finder/internal/core"
)

func TestFormatMessage(t *testing.T) {
	j := core.Job{
		Title:      "Senior Backend Engineer",
		Company:    "Stripe",
		Location:   "Remote (Worldwide)",
		Remote:     true,
		Relocation: true,
		URL:        "https://linkedin.com/jobs/1",
		PostedAt:   time.Now().Add(-3 * time.Hour),
	}
	out := formatMessage(j)
	for _, want := range []string{"Senior Backend Engineer", "Stripe", "Remote", "Relocation: Yes", "https://linkedin.com/jobs/1", "Posted:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
