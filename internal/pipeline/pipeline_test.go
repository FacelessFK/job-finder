package pipeline

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/filter"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/store"
)

type fakeProvider struct {
	name string
	jobs []core.Job
	err  error
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) SearchJobs(context.Context, core.Filters) ([]core.Job, error) {
	return f.jobs, f.err
}

type recordingProvider struct {
	onSearch func(core.Filters)
}

func (r *recordingProvider) Name() string { return "recording" }
func (r *recordingProvider) SearchJobs(_ context.Context, f core.Filters) ([]core.Job, error) {
	r.onSearch(f)
	return nil, nil
}

type fakePublisher struct {
	sent      []core.Job
	summaries []core.RunSummary
}

func (f *fakePublisher) Publish(_ context.Context, j core.Job) error {
	f.sent = append(f.sent, j)
	return nil
}

func (f *fakePublisher) PublishSummary(_ context.Context, s core.RunSummary) error {
	f.summaries = append(f.summaries, s)
	return nil
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func allowAllChain() *filter.Chain { return filter.NewChain(quiet()) }

func TestRunDedupsFiltersAndPublishes(t *testing.T) {
	jobs := []core.Job{
		{ID: "1", Title: "Backend Engineer", Company: "A", URL: "https://a.test/1", Remote: true, Description: "x"},
		{ID: "1dup", Title: "Backend Engineer", Company: "A", URL: "https://www.a.test/1/", Remote: true, Description: "x"},
		{ID: "2", Title: "Frontend Dev", Company: "B", URL: "https://b.test/2", Remote: true, Description: "y"},
	}
	prov := fakeProvider{name: "fake", jobs: jobs}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	p := New([]providers.Provider{prov}, allowAllChain(), st, pub, quiet(), core.Config{MaxPerRun: 10, DelaySeconds: 0})
	stats, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(pub.sent) != 2 {
		t.Fatalf("expected 2 published (after dedup), got %d", len(pub.sent))
	}
	if stats.Published != 2 {
		t.Errorf("stats.Published = %d", stats.Published)
	}

	// اجرای دوم: همه دیده‌شده‌اند
	pub2 := &fakePublisher{}
	p2 := New([]providers.Provider{prov}, allowAllChain(), st, pub2, quiet(), core.Config{MaxPerRun: 10, DelaySeconds: 0})
	if _, err := p2.Run(context.Background()); err != nil {
		t.Fatalf("run2: %v", err)
	}
	if len(pub2.sent) != 0 {
		t.Errorf("expected 0 on second run, got %d", len(pub2.sent))
	}
}

func TestRunIsolatesProviderError(t *testing.T) {
	good := fakeProvider{name: "good", jobs: []core.Job{{ID: "1", Title: "Dev", Company: "A", URL: "https://a.test/1", Remote: true, Description: "x"}}}
	bad := fakeProvider{name: "bad", err: context.DeadlineExceeded}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	p := New([]providers.Provider{good, bad}, allowAllChain(), st, pub, quiet(), core.Config{MaxPerRun: 10})
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run should not fail on one provider error: %v", err)
	}
	if len(pub.sent) != 1 {
		t.Errorf("expected 1 published from good provider, got %d", len(pub.sent))
	}
}

func TestRunPublishesSummaryWhenNothingIsNew(t *testing.T) {
	prov := fakeProvider{name: "fake", jobs: nil}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	cfg := core.Config{MaxPerRun: 10, Summary: core.SummaryAlways}
	p := New([]providers.Provider{prov}, allowAllChain(), st, pub, quiet(), cfg)
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(pub.summaries) != 1 {
		t.Fatalf("expected exactly 1 summary even with 0 jobs, got %d", len(pub.summaries))
	}
	if pub.summaries[0].Published != 0 {
		t.Errorf("summary.Published = %d, want 0", pub.summaries[0].Published)
	}
}

func TestSummaryReportsProviderErrors(t *testing.T) {
	bad := fakeProvider{name: "bad", err: context.DeadlineExceeded}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	p := New([]providers.Provider{bad}, allowAllChain(), st, pub, quiet(), core.Config{MaxPerRun: 10})
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(pub.summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(pub.summaries))
	}
	if len(pub.summaries[0].Errors) == 0 {
		t.Error("summary should carry the provider error so a silent run is impossible")
	}
}

func TestSummaryModeOnChangeStaysQuietWhenNothingHappened(t *testing.T) {
	prov := fakeProvider{name: "fake", jobs: nil}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	cfg := core.Config{MaxPerRun: 10, Summary: core.SummaryOnChange}
	p := New([]providers.Provider{prov}, allowAllChain(), st, pub, quiet(), cfg)
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(pub.summaries) != 0 {
		t.Errorf("onChange should stay quiet on an empty run, got %d", len(pub.summaries))
	}
}

func TestSummaryModeOnChangeReportsErrors(t *testing.T) {
	bad := fakeProvider{name: "bad", err: context.DeadlineExceeded}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	cfg := core.Config{MaxPerRun: 10, Summary: core.SummaryOnChange}
	p := New([]providers.Provider{bad}, allowAllChain(), st, pub, quiet(), cfg)
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(pub.summaries) != 1 {
		t.Errorf("onChange must still report failures, got %d summaries", len(pub.summaries))
	}
}

func TestSummaryModeNeverStaysQuietEvenOnError(t *testing.T) {
	bad := fakeProvider{name: "bad", err: context.DeadlineExceeded}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	cfg := core.Config{MaxPerRun: 10, Summary: core.SummaryNever}
	p := New([]providers.Provider{bad}, allowAllChain(), st, pub, quiet(), cfg)
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(pub.summaries) != 0 {
		t.Errorf("never should send nothing, got %d", len(pub.summaries))
	}
}

func TestRotationLimitsCountriesPassedToProvider(t *testing.T) {
	var gotCountries []string
	prov := &recordingProvider{onSearch: func(f core.Filters) { gotCountries = f.Countries }}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	cfg := core.Config{
		MaxPerRun: 10,
		Filters:   core.Filters{Countries: []string{"US", "CA", "GB", "DE"}},
		Rotation:  core.Rotation{SlotHours: 4, CountriesPerRun: 1},
	}
	p := New([]providers.Provider{prov}, allowAllChain(), st, pub, quiet(), cfg)
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(gotCountries) != 1 {
		t.Fatalf("provider got %d countries, want 1 (rotation active): %v", len(gotCountries), gotCountries)
	}
}

func TestNoRotationPassesEveryCountry(t *testing.T) {
	var gotCountries []string
	prov := &recordingProvider{onSearch: func(f core.Filters) { gotCountries = f.Countries }}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	cfg := core.Config{
		MaxPerRun: 10,
		Filters:   core.Filters{Countries: []string{"US", "CA", "GB", "DE"}},
	}
	p := New([]providers.Provider{prov}, allowAllChain(), st, pub, quiet(), cfg)
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(gotCountries) != 4 {
		t.Errorf("without rotation all 4 countries should be searched, got %v", gotCountries)
	}
}
