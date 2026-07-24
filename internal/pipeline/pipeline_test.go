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

type fakePublisher struct{ sent []core.Job }

func (f *fakePublisher) Publish(_ context.Context, j core.Job) error {
	f.sent = append(f.sent, j)
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
