package jsearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
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
