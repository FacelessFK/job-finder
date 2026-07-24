package normalize

import (
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

func TestCollapsesAndDetectsRemote(t *testing.T) {
	j := Job(core.Job{
		Title:       "  Backend   Engineer ",
		Company:     "Acme\tInc",
		Description: "This is a fully remote position.",
	})
	if j.Title != "Backend Engineer" {
		t.Errorf("title: %q", j.Title)
	}
	if j.Company != "Acme Inc" {
		t.Errorf("company: %q", j.Company)
	}
	if !j.Remote {
		t.Errorf("expected remote detected")
	}
}

func TestDetectsRelocationAndEmployment(t *testing.T) {
	j := Job(core.Job{
		Description:    "We offer visa sponsorship and relocation package.",
		EmploymentType: "Full-time",
	})
	if !j.Relocation {
		t.Errorf("expected relocation detected")
	}
	if j.EmploymentType != "FULLTIME" {
		t.Errorf("employment: %q", j.EmploymentType)
	}
}
