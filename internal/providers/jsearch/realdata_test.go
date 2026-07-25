package jsearch

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/filter"
	"github.com/aghaie/job-finder/internal/normalize"
)

// پاسخ واقعی ذخیره‌شده از JSearch v5 برای query="remote developer"، country=de.
// سرور جعلی این شکل‌ها را نشان نمی‌داد: نوع استخدام بومی‌شده، کشور خالی،
// و نبودِ کامل زمان انتشار.
const realResponse = "testdata/search_de_remote.json"

func loadReal(t *testing.T) []core.Job {
	t.Helper()
	body, err := os.ReadFile(realResponse)
	if err != nil {
		t.Fatalf("read %s: %v", realResponse, err)
	}
	jobs, err := parse(body, "de")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("captured response parsed to zero jobs")
	}
	return jobs
}

func TestRealResponseNormalizesEmploymentType(t *testing.T) {
	for _, j := range loadReal(t) {
		switch j.EmploymentType {
		case "FULLTIME", "PARTTIME", "CONTRACTOR", "INTERN", "":
		default:
			t.Errorf("job %q has unnormalized employment type %q (localized string leaked through)",
				j.Title, j.EmploymentType)
		}
	}
}

func TestRealResponseAlwaysHasCountry(t *testing.T) {
	for _, j := range loadReal(t) {
		if j.Country == "" {
			t.Errorf("job %q has no country; the searched-country fallback did not apply", j.Title)
		}
	}
}

func TestRealResponseHasUsableLocationAndURL(t *testing.T) {
	for _, j := range loadReal(t) {
		if j.Location == "" {
			t.Errorf("job %q has no location", j.Title)
		}
		if j.URL == "" {
			t.Errorf("job %q has no apply URL", j.Title)
		}
	}
}

// با تنظیمات واقعی، یک صفحه‌ی واقعی باید چند آگهی بدهد نه صفر.
// این تست همان رگرسیونی است که «صفر آگهی» را زودتر لو می‌دهد.
func TestRealResponseYieldsPublishableJobs(t *testing.T) {
	cfg := core.Config{
		MinDescriptionRunes: 200,
		Filters: core.Filters{
			Countries:                 []string{"DE"},
			Keywords:                  []string{"developer", "engineer", "designer", "product manager"},
			ExcludeKeywords:           []string{"sales"},
			EmploymentTypes:           []string{"FULLTIME", "CONTRACTOR"},
			RequireRemoteOrRelocation: true,
			PostedWithinHours:         96,
		},
	}
	chain := filter.BuildRuleChain(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	var passed int
	for _, j := range loadReal(t) {
		if chain.Allow(context.Background(), normalize.Job(j)) {
			passed++
		}
	}
	if passed == 0 {
		t.Fatal("no job from a real page survives the filters; the pipeline would be silent")
	}
	t.Logf("%d jobs published out of a real 10-job page", passed)
}

// عبارت جست‌وجو و کلیدواژه‌ی فیلتر یک چیز نیستند: «remote developer» را
// باید به API فرستاد، ولی به‌عنوان زیررشته‌ی اجباری در عنوان، تقریباً
// همه‌ی آگهی‌های واقعی را رد می‌کند.
func TestSearchQueriesAreSeparateFromFilterKeywords(t *testing.T) {
	cfg := core.Config{
		MinDescriptionRunes: 200,
		Filters: core.Filters{
			Countries:                 []string{"DE"},
			SearchQueries:             []string{"remote developer", "remote engineer"},
			Keywords:                  []string{"developer", "engineer", "entwickler"},
			EmploymentTypes:           []string{"FULLTIME", "CONTRACTOR"},
			RequireRemoteOrRelocation: true,
			PostedWithinHours:         96,
		},
	}
	chain := filter.BuildRuleChain(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	var passed int
	for _, j := range loadReal(t) {
		if chain.Allow(context.Background(), normalize.Job(j)) {
			passed++
		}
	}
	if passed == 0 {
		t.Fatal("filter keywords must stay broad; search phrases must not leak into filtering")
	}
	t.Logf("%d jobs published with search phrases and filter keywords separated", passed)
}
