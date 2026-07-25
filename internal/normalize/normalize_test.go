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

func TestDetectsWiderRelocationSignals(t *testing.T) {
	phrases := []string{
		"we offer visa sponsorship",
		"visa sponsor available for the right candidate",
		"relocation assistance provided",
		"we provide work permit support",
		"sponsorship available",
		"help with relocation",
		"eligible for skilled worker visa",
	}
	for _, p := range phrases {
		j := core.Job{Title: "Engineer", Description: p}
		if !Job(j).Relocation {
			t.Errorf("phrase %q should be detected as relocation", p)
		}
	}
}

func TestDoesNotFlagRelocationOnUnrelatedText(t *testing.T) {
	j := core.Job{Title: "Engineer", Description: "you will relocate data between clusters"}
	if Job(j).Relocation {
		t.Error("unrelated use of relocate should not set Relocation")
	}
}
