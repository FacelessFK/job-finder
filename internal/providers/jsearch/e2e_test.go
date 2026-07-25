package jsearch

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/filter"
	"github.com/aghaie/job-finder/internal/pipeline"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/ratelimit"
	"github.com/aghaie/job-finder/internal/rotate"
	"github.com/aghaie/job-finder/internal/store"
)

type capturePub struct {
	jobs      []core.Job
	summaries []core.RunSummary
}

func (c *capturePub) Publish(_ context.Context, j core.Job) error {
	c.jobs = append(c.jobs, j)
	return nil
}

func (c *capturePub) PublishSummary(_ context.Context, s core.RunSummary) error {
	c.summaries = append(c.summaries, s)
	return nil
}

// TestFullDayOfRotatedRuns یک شبانه‌روز کامل را شبیه‌سازی می‌کند: شش اجرا با
// فاصله‌ی چهار ساعت، هر اجرا یک کشور، و کلیدهایی که وسط کار سهمیه‌شان
// تمام می‌شود. مسیر کامل چرخش کشور، چرخش کلید، فیلتر و انتشار سنجیده می‌شود.
func TestFullDayOfRotatedRuns(t *testing.T) {
	var (
		mu            sync.Mutex
		requestsByKey = map[string]int{}
		countriesSeen = map[string]bool{}
	)

	const quotaPerKey = 8

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-RapidAPI-Key")
		country := r.URL.Query().Get("country")
		query := r.URL.Query().Get("query")

		mu.Lock()
		requestsByKey[key]++
		n := requestsByKey[key]
		countriesSeen[country] = true
		mu.Unlock()

		if n > quotaPerKey {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message":"You have exceeded the MONTHLY quota for Requests on your current plan, BASIC."}`))
			return
		}
		// عنوان و شرکت باید بین پرس‌وجوها فرق کند، وگرنه dedup آنها را
		// به‌درستی یکی می‌کند و تست به‌جای لوله‌کشی، dedup را می‌سنجد.
		fmt.Fprintf(w, `{"status":"OK","data":{"jobs":[{
			"job_id":"%s-%d","job_title":"Senior %s","employer_name":"Acme %s %d",
			"job_country":"%s","job_is_remote":true,"job_employment_type":"FULLTIME",
			"job_description":"%s","job_apply_link":"https://jobs.test/%s/%d",
			"job_posted_at_datetime_utc":"%s"}]}}`,
			country, n, query, query, n, strings.ToUpper(country),
			strings.Repeat("we build great products. ", 20),
			country, n, time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339))
	}))
	defer srv.Close()

	countries := []string{
		"US", "CA", "GB", "IE", "DE", "NL", "FR", "ES", "IT", "PT",
		"SE", "DK", "NO", "FI", "CH", "AT", "BE", "PL", "CZ", "AU",
	}
	keywords := []string{"developer", "engineer", "designer", "product manager"}

	seenPath := t.TempDir() + "/seen.json"
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pub := &capturePub{}
	base := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	const slots = 6
	for slot := 0; slot < slots; slot++ {
		now := base.Add(time.Duration(slot*4) * time.Hour)
		active := rotate.Slice(countries, now, 4, 1)

		cfg := core.Config{
			Filters: core.Filters{
				Countries:                 active,
				Keywords:                  keywords,
				EmploymentTypes:           []string{"FULLTIME", "CONTRACTOR"},
				RequireRemoteOrRelocation: true,
				PostedWithinHours:         96,
			},
			NumPages: 1, MaxPerRun: 25, MinDescriptionRunes: 200,
			Summary: core.SummaryOnChange,
		}

		prov := &Provider{
			keys: []string{"k1", "k2", "k3", "k4"}, baseURL: srv.URL,
			client: srv.Client(), numPages: 1, retired: map[int]bool{},
			limiter: ratelimit.New(0),
		}
		st, err := store.NewFileStore(seenPath, 5000)
		if err != nil {
			t.Fatalf("store: %v", err)
		}
		p := pipeline.New([]providers.Provider{prov}, filter.BuildRuleChain(log, cfg),
			st, pub, log, cfg)
		if _, err := p.Run(context.Background()); err != nil {
			t.Fatalf("slot %d: %v", slot, err)
		}
	}

	t.Logf("countries searched: %d %v", len(countriesSeen), keysOf(countriesSeen))
	t.Logf("requests per key:   %v", requestsByKey)
	t.Logf("jobs published:     %d", len(pub.jobs))

	if len(countriesSeen) != slots {
		t.Errorf("expected %d distinct countries over %d slots, got %d: %v",
			slots, slots, len(countriesSeen), keysOf(countriesSeen))
	}
	// ۶ اسلات × ۴ کلیدواژه = ۲۴ درخواست، با سقف ۸ در هر کلید.
	if len(requestsByKey) < 3 {
		t.Errorf("key rotation never kicked in, keys used: %v", requestsByKey)
	}
	if len(pub.jobs) != slots*len(keywords) {
		t.Errorf("published %d jobs, want %d", len(pub.jobs), slots*len(keywords))
	}
	for _, j := range pub.jobs {
		if j.Country == "" {
			t.Errorf("published job without country: %+v", j)
		}
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
