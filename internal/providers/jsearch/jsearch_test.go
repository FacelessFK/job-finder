package jsearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/ratelimit"
)

func TestSearchMapsJobs(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-RapidAPI-Key")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j1","job_title":"Backend Engineer","employer_name":"Acme",
			"job_city":"Berlin","job_country":"DE","job_is_remote":true,
			"job_description":"desc","job_apply_link":"https://acme.test/1",
			"job_employment_type":"FULLTIME","job_posted_at_datetime_utc":"2026-07-24T10:00:00.000Z"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client()}
	jobs, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"backend"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	if j.Title != "Backend Engineer" || j.Company != "Acme" || j.Source != "jsearch" {
		t.Errorf("bad mapping: %+v", j)
	}
	if !j.Remote || j.URL != "https://acme.test/1" {
		t.Errorf("bad fields: %+v", j)
	}
	if gotKey != "k" {
		t.Errorf("key header: %q", gotKey)
	}
}

func TestSearchQueriesEachCountry(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Query().Get("country"))
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	_, err := p.SearchJobs(context.Background(), core.Filters{
		Keywords:  []string{"developer"},
		Countries: []string{"US", "AU"},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	want := []string{"us", "au"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("country params = %v, want %v", got, want)
	}
}

func TestSearchOmitsCountryWhenNoneConfigured(t *testing.T) {
	var calls int
	var hadCountry bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, hadCountry = r.URL.Query()["country"]
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	if _, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
	if hadCountry {
		t.Error("country param should be absent when no countries configured")
	}
}

func TestSearchSendsConfiguredNumPages(t *testing.T) {
	var gotPages string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPages = r.URL.Query().Get("num_pages")
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 3}
	if _, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if gotPages != "3" {
		t.Errorf("num_pages = %q, want \"3\"", gotPages)
	}
}

func TestSearchMapsCountryCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j1","job_title":"Dev","employer_name":"Acme",
			"job_city":"Sydney","job_country":"AU","job_apply_link":"https://a.test/1"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	jobs, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if jobs[0].Country != "AU" {
		t.Errorf("Country = %q, want \"AU\"", jobs[0].Country)
	}
}

func TestSearchDeduplicatesAcrossCountryQueries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"same","job_title":"Dev","employer_name":"Acme",
			"job_country":"US","job_apply_link":"https://a.test/1"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	jobs, err := p.SearchJobs(context.Background(), core.Filters{
		Keywords:  []string{"dev", "engineer"},
		Countries: []string{"US", "DE"},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected identical job collapsed to 1, got %d", len(jobs))
	}
}

func TestSearchContinuesWhenOneQueryFails(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if r.URL.Query().Get("country") == "us" {
			// ۴۰۰ یعنی خودِ این پرس‌وجو بد است، نه اینکه کلید سوخته باشد.
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j2","job_title":"Dev","employer_name":"B",
			"job_country":"AU","job_apply_link":"https://b.test/2"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	jobs, err := p.SearchJobs(context.Background(), core.Filters{
		Keywords:  []string{"dev"},
		Countries: []string{"US", "AU"},
	})
	if err == nil {
		t.Error("expected an error describing the failed country query")
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job from the country that succeeded, got %d", len(jobs))
	}
}

func TestSearchSpacesOutRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{
		keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1,
		limiter: ratelimit.New(50 * time.Millisecond),
	}
	start := time.Now()
	if _, err := p.SearchJobs(context.Background(), core.Filters{
		Keywords: []string{"a", "b", "c"},
	}); err != nil {
		t.Fatalf("search: %v", err)
	}
	// سه درخواست با فاصله‌ی ۵۰ms یعنی دست‌کم دو بار انتظار.
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("requests were not rate limited: elapsed %v", elapsed)
	}
}

// quotaBody همان چیزی است که RapidAPI هنگام تمام‌شدن سهمیه‌ی ماهانه برمی‌گرداند.
const quotaBody = `{"message":"You have exceeded the MONTHLY quota for Requests on your current plan, BASIC."}`

func TestSwitchesToNextKeyWhenQuotaExhausted(t *testing.T) {
	var usedKeys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := r.Header.Get("X-RapidAPI-Key")
		usedKeys = append(usedKeys, k)
		if k == "k1" {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(quotaBody))
			return
		}
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j1","job_title":"Dev","employer_name":"Acme",
			"job_country":"US","job_apply_link":"https://a.test/1"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k1", "k2"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	jobs, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}})
	if err != nil {
		t.Fatalf("search should succeed on the second key: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if len(usedKeys) < 2 || usedKeys[0] != "k1" || usedKeys[1] != "k2" {
		t.Errorf("expected fallthrough k1 then k2, got %v", usedKeys)
	}
}

func TestDoesNotRetryExhaustedKeyOnLaterQueries(t *testing.T) {
	var usedKeys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := r.Header.Get("X-RapidAPI-Key")
		usedKeys = append(usedKeys, k)
		if k == "k1" {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(quotaBody))
			return
		}
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k1", "k2"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	if _, err := p.SearchJobs(context.Background(), core.Filters{
		Keywords: []string{"a", "b", "c"},
	}); err != nil {
		t.Fatalf("search: %v", err)
	}
	// k1 فقط یک‌بار امتحان می‌شود؛ بعد از آن باید کنار گذاشته شود.
	var k1Count int
	for _, k := range usedKeys {
		if k == "k1" {
			k1Count++
		}
	}
	if k1Count != 1 {
		t.Errorf("exhausted key was retried %d times, want 1", k1Count)
	}
}

func TestFailsWhenAllKeysExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(quotaBody))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k1", "k2"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	_, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}})
	if err == nil {
		t.Fatal("expected an error when every key is exhausted")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Errorf("error should name the exhaustion cause, got: %v", err)
	}
}

func TestSwitchesKeyOnForbidden(t *testing.T) {
	var usedKeys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := r.Header.Get("X-RapidAPI-Key")
		usedKeys = append(usedKeys, k)
		if k == "bad" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"You are not subscribed to this API."}`))
			return
		}
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"bad", "good"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	if _, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}}); err != nil {
		t.Fatalf("an invalid key should fall through to the next: %v", err)
	}
	if len(usedKeys) < 2 || usedKeys[1] != "good" {
		t.Errorf("expected fallthrough to good key, got %v", usedKeys)
	}
}

func TestRateLimitBurstStillRetriesSameKey(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// ۴۲۹ بدون پیام سهمیه یعنی سقف لحظه‌ای، نه تمام‌شدن ماهانه.
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message":"Too many requests"}`))
			return
		}
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"only"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	if _, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}}); err != nil {
		t.Fatalf("a burst 429 should be retried on the same key: %v", err)
	}
	if attempts < 2 {
		t.Errorf("expected a retry, got %d attempts", attempts)
	}
}

// پاسخ‌های واقعی v5: نوع استخدام در فیلد مفرد بومی‌شده است ("Vollzeit")
// و کد استاندارد فقط در آرایه‌ی job_employment_types می‌آید.
func TestPrefersNormalizedEmploymentTypesArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j1","job_title":"Software Developer (m/w/d)",
			"employer_name":"Academic Work","job_employment_type":"Vollzeit",
			"job_employment_types":["FULLTIME"],
			"job_apply_link":"https://a.test/1"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	jobs, _ := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}})
	if jobs[0].EmploymentType != "FULLTIME" {
		t.Errorf("EmploymentType = %q, want FULLTIME (from the array, not the localized string)", jobs[0].EmploymentType)
	}
}

// وقتی job_country خالی است، کشوری که جست‌وجو کردیم را می‌دانیم.
func TestFallsBackToSearchedCountry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j1","job_title":"Dev","employer_name":"A",
			"job_country":null,"job_apply_link":"https://a.test/1"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	jobs, _ := p.SearchJobs(context.Background(), core.Filters{
		Keywords: []string{"dev"}, Countries: []string{"DE"},
	})
	if jobs[0].Country != "DE" {
		t.Errorf("Country = %q, want DE from the search parameter", jobs[0].Country)
	}
}

// job_location تنها جایی است که مکان واقعی می‌آید وقتی city/country خالی‌اند.
func TestUsesJobLocationWhenCityAndCountryMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j1","job_title":"Dev","employer_name":"A",
			"job_city":null,"job_country":null,
			"job_location":"Gräfelfing     •  über Talent.de",
			"job_apply_link":"https://a.test/1"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	jobs, _ := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}})
	if !strings.Contains(jobs[0].Location, "Gräfelfing") {
		t.Errorf("Location = %q, want the job_location value", jobs[0].Location)
	}
}

func TestParsesPostedAtFromTimestampFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j1","job_title":"Dev","employer_name":"A",
			"job_posted_at_datetime_utc":null,"job_posted_at_timestamp":1769000000,
			"job_apply_link":"https://a.test/1"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	jobs, _ := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"dev"}})
	if jobs[0].PostedAt.IsZero() {
		t.Error("PostedAt should fall back to job_posted_at_timestamp")
	}
	if got := jobs[0].PostedAt.UTC().Unix(); got != 1769000000 {
		t.Errorf("PostedAt unix = %d, want 1769000000", got)
	}
}

// SearchQueries تعیین می‌کند چه چیزی به API می‌رود؛ Keywords فقط فیلتر است.
func TestSendsSearchQueriesNotFilterKeywords(t *testing.T) {
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = append(sent, r.URL.Query().Get("query"))
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	_, err := p.SearchJobs(context.Background(), core.Filters{
		SearchQueries: []string{"remote developer"},
		Keywords:      []string{"developer", "engineer", "designer"},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(sent) != 1 || sent[0] != "remote developer" {
		t.Errorf("queries sent = %v, want [remote developer]", sent)
	}
}

func TestFallsBackToKeywordsWhenNoSearchQueries(t *testing.T) {
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = append(sent, r.URL.Query().Get("query"))
		w.Write([]byte(`{"status":"OK","data":{"jobs":[]}}`))
	}))
	defer srv.Close()

	p := &Provider{keys: []string{"k"}, baseURL: srv.URL, client: srv.Client(), numPages: 1}
	if _, err := p.SearchJobs(context.Background(), core.Filters{
		Keywords: []string{"golang", "rust"},
	}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(sent) != 2 {
		t.Errorf("queries sent = %v, want the two keywords", sent)
	}
}
