package dedup

import (
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

func TestDedupeByURL(t *testing.T) {
	jobs := []core.Job{
		{Title: "A", Company: "X", URL: "https://ex.com/1"},
		{Title: "B", Company: "Y", URL: "https://www.ex.com/1/"},
		{Title: "C", Company: "Z", URL: "https://ex.com/2"},
	}
	out := Dedupe(jobs)
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d", len(out))
	}
}

func TestDedupeBySimilarTitleCompany(t *testing.T) {
	jobs := []core.Job{
		{Title: "Senior Backend Engineer", Company: "Stripe"},
		{Title: "Senior Backend Engineer", Company: "Stripe"},
		{Title: "Frontend Developer", Company: "Vercel"},
	}
	out := Dedupe(jobs)
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d", len(out))
	}
}
