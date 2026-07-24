// Package normalize خروجی هر Provider را به شکل استاندارد درمی‌آورد.
package normalize

import (
	"strings"

	"github.com/aghaie/job-finder/internal/core"
)

var remoteSignals = []string{
	"fully remote", "100% remote", "work from anywhere",
	"remote-first", "remote first", "fully-remote",
}

var relocationSignals = []string{
	"visa sponsorship", "relocation package", "relocation assistance",
	"relocation support", "we sponsor",
}

// Job یک جاب را نرمال می‌کند: پاک‌سازی فاصله‌ها، استانداردسازی نوع/سطح، و
// تشخیص اولیه‌ی Remote/Relocation از روی متن.
func Job(j core.Job) core.Job {
	j.Title = collapse(j.Title)
	j.Company = collapse(j.Company)
	j.Location = collapse(j.Location)
	j.Description = collapse(j.Description)
	j.EmploymentType = normEmployment(j.EmploymentType)
	j.Seniority = normSeniority(j.Seniority)

	hay := strings.ToLower(j.Title + " " + j.Location + " " + j.Description)
	if !j.Remote && containsAny(hay, remoteSignals) {
		j.Remote = true
	}
	if !j.Relocation && containsAny(hay, relocationSignals) {
		j.Relocation = true
	}
	return j
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func containsAny(hay string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(hay, n) {
			return true
		}
	}
	return false
}

func normEmployment(s string) string {
	l := strings.ToLower(strings.TrimSpace(s))
	l = strings.ReplaceAll(l, "-", "")
	l = strings.ReplaceAll(l, "_", "")
	l = strings.ReplaceAll(l, " ", "")
	switch {
	case l == "":
		return ""
	case strings.Contains(l, "intern"):
		return "INTERN"
	case strings.Contains(l, "part"):
		return "PARTTIME"
	case strings.Contains(l, "contract"), strings.Contains(l, "freelance"):
		return "CONTRACTOR"
	case strings.Contains(l, "full"):
		return "FULLTIME"
	default:
		return strings.ToUpper(s)
	}
}

func normSeniority(s string) string {
	l := strings.ToLower(strings.TrimSpace(s))
	switch {
	case l == "":
		return ""
	case strings.Contains(l, "lead"), strings.Contains(l, "principal"), strings.Contains(l, "staff"):
		return "lead"
	case strings.Contains(l, "senior"), strings.Contains(l, "sr"):
		return "senior"
	case strings.Contains(l, "junior"), strings.Contains(l, "jr"), strings.Contains(l, "entry"):
		return "junior"
	case strings.Contains(l, "mid"), strings.Contains(l, "intermediate"):
		return "mid"
	default:
		return l
	}
}
