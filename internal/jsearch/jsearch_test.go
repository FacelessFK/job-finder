package jsearch

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseSearchResponse(t *testing.T) {
	body := `{
	  "status": "OK",
	  "data": [
	    {
	      "job_id": "abc123",
	      "job_title": "Backend Developer",
	      "employer_name": "Acme GmbH",
	      "job_city": "Berlin",
	      "job_country": "DE",
	      "job_is_remote": true,
	      "job_description": "We build things.",
	      "job_apply_link": "https://example.com/apply",
	      "job_posted_at_datetime_utc": "2026-07-24T10:00:00.000Z"
	    }
	  ]
	}`
	jobs, err := parseSearchResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	if j.ID != "abc123" {
		t.Errorf("ID: got %q", j.ID)
	}
	if j.Title != "Backend Developer" {
		t.Errorf("Title: got %q", j.Title)
	}
	if j.Company != "Acme GmbH" {
		t.Errorf("Company: got %q", j.Company)
	}
	if !j.IsRemote {
		t.Errorf("IsRemote: expected true")
	}
	if j.ApplyLink != "https://example.com/apply" {
		t.Errorf("ApplyLink: got %q", j.ApplyLink)
	}
}

func TestFetchUsesQueryAndKey(t *testing.T) {
	var gotQuery, gotKey, gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		gotKey = r.Header.Get("X-RapidAPI-Key")
		gotHost = r.Header.Get("X-RapidAPI-Host")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"OK","data":[]}`))
	}))
	defer srv.Close()

	c := &Client{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := c.Fetch("golang developer", "today", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "golang developer" {
		t.Errorf("query: got %q", gotQuery)
	}
	if gotKey != "test-key" {
		t.Errorf("key header: got %q", gotKey)
	}
	if gotHost != "jsearch.p.rapidapi.com" {
		t.Errorf("host header: got %q", gotHost)
	}
}
