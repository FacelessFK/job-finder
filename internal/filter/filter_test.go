package filter

import (
	"context"
	"io"
	"log/slog"
	"strings"
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
	cfg := baseCfg()
	cfg.Filters.RequireRemoteOrRelocation = true
	c := BuildRuleChain(quietLog(), cfg)
	neither := core.Job{Title: "Engineer", Description: "a long enough description here"}
	if c.Allow(context.Background(), neither) {
		t.Error("job that is neither remote nor relocation should be rejected when required")
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

func TestChainRejectsCountryOutsideList(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.Countries = []string{"US", "DE"}
	c := BuildRuleChain(quietLog(), cfg)
	desc := "a long enough description here"

	if !c.Allow(context.Background(), core.Job{Title: "Engineer", Country: "DE", Description: desc}) {
		t.Error("job in an allowed country should pass")
	}
	if !c.Allow(context.Background(), core.Job{Title: "Engineer", Country: "us", Description: desc}) {
		t.Error("country match should be case-insensitive")
	}
	if c.Allow(context.Background(), core.Job{Title: "Engineer", Country: "IN", Description: desc}) {
		t.Error("job outside the country list should be rejected")
	}
}

func TestChainAllowsUnknownCountry(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.Countries = []string{"US"}
	c := BuildRuleChain(quietLog(), cfg)
	j := core.Job{Title: "Engineer", Country: "", Description: "a long enough description here"}
	if !c.Allow(context.Background(), j) {
		t.Error("job with unknown country should not be dropped")
	}
}

func TestChainAllowsOnsiteWhenNotRequired(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.RequireRemoteOrRelocation = false
	c := BuildRuleChain(quietLog(), cfg)
	onsite := core.Job{Title: "Engineer", Description: "a long enough description here"}
	if !c.Allow(context.Background(), onsite) {
		t.Error("onsite job should pass when remote/relocation is not required")
	}
}

func TestChainRejectsDisallowedSeniority(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.Seniority = []string{"mid", "senior"}
	c := BuildRuleChain(quietLog(), cfg)
	desc := "a long enough description here"

	if !c.Allow(context.Background(), core.Job{Title: "Engineer", Seniority: "senior", Description: desc}) {
		t.Error("senior job should pass")
	}
	if c.Allow(context.Background(), core.Job{Title: "Engineer", Seniority: "junior", Description: desc}) {
		t.Error("junior job should be rejected")
	}
}

func TestChainAllowsUnknownSeniority(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.Seniority = []string{"mid", "senior"}
	c := BuildRuleChain(quietLog(), cfg)
	j := core.Job{Title: "Engineer", Seniority: "", Description: "a long enough description here"}
	if !c.Allow(context.Background(), j) {
		t.Error("job with unknown seniority should not be dropped")
	}
}

// realWorldCfg همان تنظیمات واقعی پروژه است تا رفتار نهایی سنجیده شود.
func realWorldCfg() core.Config {
	return core.Config{
		MinDescriptionRunes: 200,
		Filters: core.Filters{
			Countries:                 []string{"US", "CA", "GB", "DE", "NL", "IE", "AU"},
			Keywords:                  []string{"developer", "engineer", "designer", "product manager"},
			ExcludeKeywords:           []string{"sales"},
			EmploymentTypes:           []string{"FULLTIME", "CONTRACTOR"},
			Seniority:                 []string{"mid", "senior", "lead"},
			RequireRemoteOrRelocation: true,
			PostedWithinHours:         48,
		},
	}
}

func longDesc(extra string) string {
	return extra + " " + strings.Repeat("we build great software products. ", 10)
}

func TestRealConfigAcceptsTargetRegions(t *testing.T) {
	c := BuildRuleChain(quietLog(), realWorldCfg())
	now := time.Now().Add(-3 * time.Hour)

	cases := []struct {
		name string
		job  core.Job
		want bool
	}{
		{"australian remote engineer", core.Job{
			Title: "Senior Backend Engineer", Country: "AU", Remote: true,
			EmploymentType: "FULLTIME", Seniority: "senior",
			Description: longDesc("Fully remote role."), PostedAt: now}, true},
		{"german onsite with visa sponsorship", core.Job{
			Title: "Product Designer", Country: "DE", Remote: false, Relocation: true,
			EmploymentType: "FULLTIME", Seniority: "mid",
			Description: longDesc("We offer visa sponsorship and relocation package."), PostedAt: now}, true},
		{"canadian remote product manager", core.Job{
			Title: "Product Manager", Country: "CA", Remote: true,
			EmploymentType: "FULLTIME", Description: longDesc("Remote across Canada."), PostedAt: now}, true},
		{"indian remote engineer outside target list", core.Job{
			Title: "Backend Engineer", Country: "IN", Remote: true,
			EmploymentType: "FULLTIME", Description: longDesc("Remote role."), PostedAt: now}, false},
		{"us onsite without any relocation support", core.Job{
			Title: "Software Engineer", Country: "US", Remote: false,
			EmploymentType: "FULLTIME", Description: longDesc("Onsite in Austin."), PostedAt: now}, false},
	}

	for _, tc := range cases {
		if got := c.Allow(context.Background(), tc.job); got != tc.want {
			t.Errorf("%s: Allow = %v, want %v", tc.name, got, tc.want)
		}
	}
}
