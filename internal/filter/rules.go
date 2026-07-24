package filter

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/aghaie/job-finder/internal/core"
)

// BuildRuleChain زنجیره‌ی فیلترهای قانون‌محور را از روی config می‌سازد.
func BuildRuleChain(log *slog.Logger, cfg core.Config) *Chain {
	f := cfg.Filters
	return NewChain(log,
		employmentFilter{allowInternship: cfg.AllowInternship, allowed: upperAll(f.EmploymentTypes)},
		keywordFilter{keywords: lowerAll(f.Keywords), exclude: lowerAll(f.ExcludeKeywords)},
		companyFilter{whitelist: lowerAll(f.CompanyWhitelist), blacklist: lowerAll(f.CompanyBlacklist)},
		freshnessFilter{withinHours: f.PostedWithinHours},
		valueFilter{remoteOnly: f.RemoteOnly, relocationOnly: f.RelocationOnly},
		spamFilter{minRunes: cfg.MinDescriptionRunes},
	)
}

// --- Employment ---

type employmentFilter struct {
	allowInternship bool
	allowed         []string
}

func (e employmentFilter) Name() string { return "employment" }
func (e employmentFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	if j.EmploymentType == "INTERN" && !e.allowInternship {
		return reject("internship not allowed"), nil
	}
	if len(e.allowed) > 0 && j.EmploymentType != "" && !contains(e.allowed, j.EmploymentType) {
		return reject("employment type not in allowed list"), nil
	}
	return pass(), nil
}

// --- Keyword ---

type keywordFilter struct {
	keywords []string
	exclude  []string
}

func (k keywordFilter) Name() string { return "keyword" }
func (k keywordFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	hay := strings.ToLower(j.Title + " " + j.Description)
	for _, ex := range k.exclude {
		if ex != "" && strings.Contains(hay, ex) {
			return reject("matched exclude keyword: " + ex), nil
		}
	}
	if len(k.keywords) > 0 {
		for _, kw := range k.keywords {
			if kw != "" && strings.Contains(hay, kw) {
				return pass(), nil
			}
		}
		return reject("no required keyword matched"), nil
	}
	return pass(), nil
}

// --- Company ---

type companyFilter struct {
	whitelist []string
	blacklist []string
}

func (c companyFilter) Name() string { return "company" }
func (c companyFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	comp := strings.ToLower(j.Company)
	for _, b := range c.blacklist {
		if b != "" && strings.Contains(comp, b) {
			return reject("company blacklisted"), nil
		}
	}
	if len(c.whitelist) > 0 {
		for _, w := range c.whitelist {
			if w != "" && strings.Contains(comp, w) {
				return pass(), nil
			}
		}
		return reject("company not in whitelist"), nil
	}
	return pass(), nil
}

// --- Freshness ---

type freshnessFilter struct {
	withinHours int
}

func (fr freshnessFilter) Name() string { return "freshness" }
func (fr freshnessFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	if fr.withinHours <= 0 || j.PostedAt.IsZero() {
		return pass(), nil
	}
	if time.Since(j.PostedAt) > time.Duration(fr.withinHours)*time.Hour {
		return reject("older than freshness window"), nil
	}
	return pass(), nil
}

// --- Value (Remote OR Relocation) ---

type valueFilter struct {
	remoteOnly     bool
	relocationOnly bool
}

func (v valueFilter) Name() string { return "value" }
func (v valueFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	switch {
	case v.remoteOnly && v.relocationOnly:
		if j.Remote && j.Relocation {
			return pass(), nil
		}
		return reject("requires both remote and relocation"), nil
	case v.remoteOnly:
		if j.Remote {
			return pass(), nil
		}
		return reject("not remote"), nil
	case v.relocationOnly:
		if j.Relocation {
			return pass(), nil
		}
		return reject("no relocation"), nil
	default:
		if j.Remote || j.Relocation {
			return pass(), nil
		}
		return reject("neither remote nor relocation"), nil
	}
}

// --- Spam / quality ---

type spamFilter struct {
	minRunes int
}

func (s spamFilter) Name() string { return "spam" }
func (s spamFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	if s.minRunes > 0 && len([]rune(strings.TrimSpace(j.Description))) < s.minRunes {
		return reject("description too short"), nil
	}
	return pass(), nil
}

// --- helpers ---

func pass() core.Decision           { return core.Decision{Publish: true} }
func reject(r string) core.Decision { return core.Decision{Publish: false, Reason: r} }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func lowerAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToLower(strings.TrimSpace(s)))
	}
	return out
}

func upperAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToUpper(strings.TrimSpace(s)))
	}
	return out
}
