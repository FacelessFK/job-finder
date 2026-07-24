package filter

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aghaie/job-finder/internal/core"
)

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func baseCfg() core.Config {
	return core.Config{
		MinDescriptionRunes: 10,
		Filters: core.Filters{
			Keywords: []string{"engineer"},
		},
	}
}

func TestChainRejectsInternship(t *testing.T) {
	c := BuildRuleChain(quietLog(), baseCfg())
	j := core.Job{Title: "Engineer", EmploymentType: "INTERN", Description: "a long enough description here"}
	if c.Allow(context.Background(), j) {
		t.Error("internship should be rejected by default")
	}
}

func TestChainRejectsMissingKeyword(t *testing.T) {
	c := BuildRuleChain(quietLog(), baseCfg())
	j := core.Job{Title: "Sales Manager", Description: "a long enough description here"}
	if c.Allow(context.Background(), j) {
		t.Error("job without keyword should be rejected")
	}
}

func TestChainRejectsShortDescription(t *testing.T) {
	c := BuildRuleChain(quietLog(), baseCfg())
	j := core.Job{Title: "Engineer", Description: "short"}
	if c.Allow(context.Background(), j) {
		t.Error("too-short description should be rejected")
	}
}

func TestChainRemoteOnly(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.RemoteOnly = true
	c := BuildRuleChain(quietLog(), cfg)
	remote := core.Job{Title: "Engineer", Remote: true, Description: "a long enough description here"}
	onsite := core.Job{Title: "Engineer", Remote: false, Description: "a long enough description here"}
	if !c.Allow(context.Background(), remote) {
		t.Error("remote job should pass")
	}
	if c.Allow(context.Background(), onsite) {
		t.Error("non-remote job should be rejected under remoteOnly")
	}
}

func TestChainDefaultRequiresRemoteOrRelocation(t *testing.T) {
	c := BuildRuleChain(quietLog(), baseCfg())
	neither := core.Job{Title: "Engineer", Description: "a long enough description here"}
	if c.Allow(context.Background(), neither) {
		t.Error("job that is neither remote nor relocation should be rejected by default")
	}
	reloc := core.Job{Title: "Engineer", Relocation: true, Description: "a long enough description here"}
	if !c.Allow(context.Background(), reloc) {
		t.Error("relocation job should pass default value filter")
	}
}

func TestChainFreshness(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.PostedWithinHours = 24
	c := BuildRuleChain(quietLog(), cfg)
	old := core.Job{Title: "Engineer", Remote: true, Description: "a long enough description here", PostedAt: time.Now().Add(-48 * time.Hour)}
	if c.Allow(context.Background(), old) {
		t.Error("old job should be rejected")
	}
	fresh := core.Job{Title: "Engineer", Remote: true, Description: "a long enough description here", PostedAt: time.Now().Add(-2 * time.Hour)}
	if !c.Allow(context.Background(), fresh) {
		t.Error("fresh job should pass")
	}
}
