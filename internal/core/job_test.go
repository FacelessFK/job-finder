package core

import "testing"

func TestFingerprintPrefersURL(t *testing.T) {
	a := Job{Title: "Dev", Company: "X", URL: "https://www.example.com/jobs/123/?utm=abc"}
	b := Job{Title: "Different", Company: "Y", URL: "https://example.com/jobs/123"}
	if a.Fingerprint() != b.Fingerprint() {
		t.Errorf("same normalized URL should match:\n%s\n%s", a.Fingerprint(), b.Fingerprint())
	}
}

func TestFingerprintFallsBackToCompanyTitle(t *testing.T) {
	a := Job{Title: "Backend Dev", Company: "Acme"}
	b := Job{Title: "backend dev", Company: "  ACME "}
	if a.Fingerprint() != b.Fingerprint() {
		t.Errorf("company+title should be case/space-insensitive")
	}
	c := Job{Title: "Other", Company: "Acme"}
	if a.Fingerprint() == c.Fingerprint() {
		t.Errorf("different titles must differ")
	}
}
