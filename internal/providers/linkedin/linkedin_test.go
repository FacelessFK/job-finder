package linkedin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

func TestSearchMapsJobs(t *testing.T) {
	var gotHost, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Header.Get("X-RapidAPI-Host")
		gotKey = r.Header.Get("X-RapidAPI-Key")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{
			"id":"li1","title":"Senior Go Engineer","company_name":"Stripe",
			"location":"Remote","url":"https://linkedin.com/jobs/li1","is_remote":true,
			"employment_type":"Full-time","seniority_level":"Senior",
			"description":"great role","posted_at":"2026-07-24T09:00:00Z"}]}`))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	p := &Provider{key: "k", host: host, path: "/search", scheme: "http", client: srv.Client()}
	jobs, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"go"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1, got %d", len(jobs))
	}
	j := jobs[0]
	if j.Title != "Senior Go Engineer" || j.Company != "Stripe" || j.Source != "linkedin" {
		t.Errorf("bad mapping: %+v", j)
	}
	if gotKey != "k" || gotHost != host {
		t.Errorf("headers: host=%q key=%q", gotHost, gotKey)
	}
}
