package jsearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
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

	p := &Provider{key: "k", baseURL: srv.URL, client: srv.Client()}
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

	p := &Provider{key: "k", baseURL: srv.URL, client: srv.Client(), numPages: 1}
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

	p := &Provider{key: "k", baseURL: srv.URL, client: srv.Client(), numPages: 1}
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

	p := &Provider{key: "k", baseURL: srv.URL, client: srv.Client(), numPages: 3}
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

	p := &Provider{key: "k", baseURL: srv.URL, client: srv.Client(), numPages: 1}
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

	p := &Provider{key: "k", baseURL: srv.URL, client: srv.Client(), numPages: 1}
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
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte(`{"status":"OK","data":{"jobs":[{
			"job_id":"j2","job_title":"Dev","employer_name":"B",
			"job_country":"AU","job_apply_link":"https://b.test/2"}]}}`))
	}))
	defer srv.Close()

	p := &Provider{key: "k", baseURL: srv.URL, client: srv.Client(), numPages: 1}
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
		key: "k", baseURL: srv.URL, client: srv.Client(), numPages: 1,
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
