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

// پاسخ‌های واقعی آلمان پر از عبارت‌های بومی است؛ بدون این‌ها آگهی‌های
// دورکار واقعی به‌عنوان حضوری رد می‌شوند.
func TestDetectsGermanRemoteSignals(t *testing.T) {
	phrases := []string{
		"100% Homeoffice möglich",
		"Remote-Option verfügbar",
		"vollständig remote",
		"komplett remote arbeiten",
		"home office möglich",
	}
	for _, p := range phrases {
		if !Job(core.Job{Title: "Entwickler", Description: p}).Remote {
			t.Errorf("phrase %q should be detected as remote", p)
		}
	}
}

func TestDetectsGermanRelocationSignals(t *testing.T) {
	phrases := []string{
		"Wir unterstützen bei der Visum-Beantragung",
		"Arbeitserlaubnis wird gesponsert",
		"Umzugshilfe wird angeboten",
		"Relocation-Paket inklusive",
	}
	for _, p := range phrases {
		if !Job(core.Job{Title: "Entwickler", Description: p}).Relocation {
			t.Errorf("phrase %q should be detected as relocation", p)
		}
	}
}

// در عنوانِ آگهی، کلمه‌ی remote تقریبا همیشه یعنی خودِ شغل دورکار است.
// این با متنِ توضیحات فرق دارد که «remote server» هم در آن می‌آید.
func TestRemoteWordInTitleMeansRemote(t *testing.T) {
	titles := []string{
		"Remote Product Manager",
		"Remote UI/UX Designer for Software",
		"Product Manager: AI-Driven Growth & Impact (Remote)",
		"Senior Java Frontend Developer (m/f/d) - Remote",
		"Algorithm Engineer - Modern C++ (Remote)",
		"(Junior) .NET Developer (m/w/d) - remote",
	}
	for _, ti := range titles {
		if !Job(core.Job{Title: ti, Description: "x"}).Remote {
			t.Errorf("title %q should be detected as remote", ti)
		}
	}
}

func TestRemoteSensingIsNotRemoteWork(t *testing.T) {
	j := core.Job{Title: "Remote Sensing Engineer", Description: "satellite imagery"}
	if Job(j).Remote {
		t.Error("remote sensing is a field of work, not a work arrangement")
	}
}

func TestRemoteWordInDescriptionAloneIsNotEnough(t *testing.T) {
	j := core.Job{Title: "Backend Engineer", Description: "you will manage a remote server cluster"}
	if Job(j).Remote {
		t.Error("a bare remote in the description should not flag the job as remote")
	}
}
