package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/aghaie/job-finder/internal/core"
)

// formatMessage متن پیام تلگرام را از یک جاب می‌سازد.
func formatMessage(j core.Job) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🚀 %s\n", strings.TrimSpace(j.Title))
	if j.Company != "" {
		fmt.Fprintf(&b, "🏢 %s\n", strings.TrimSpace(j.Company))
	}

	loc := strings.TrimSpace(j.Location)
	if j.Remote {
		if loc == "" {
			loc = "Remote"
		} else if !strings.Contains(strings.ToLower(loc), "remote") {
			loc = "Remote — " + loc
		}
	}
	if loc != "" {
		fmt.Fprintf(&b, "🌍 %s\n", loc)
	}

	if j.Relocation {
		b.WriteString("✈️ Relocation: Yes\n")
	}
	if age := humanizeAge(j.PostedAt); age != "" {
		fmt.Fprintf(&b, "🕒 Posted: %s\n", age)
	}
	if j.URL != "" {
		fmt.Fprintf(&b, "🔗 %s\n", j.URL)
	}
	return b.String()
}

func humanizeAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		m := int(d.Minutes())
		if m < 1 {
			m = 1
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}
