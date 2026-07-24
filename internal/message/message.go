// Package message متن پیام فارسی تلگرام را از یک جاب می‌سازد.
package message

import (
	"fmt"
	"strings"

	"github.com/aghaie/job-finder/internal/jsearch"
	"github.com/aghaie/job-finder/internal/summarize"
)

// Build متن پیام را برای یک جاب می‌سازد.
func Build(j jsearch.Job, sum summarize.Summarizer) string {
	var b strings.Builder

	fmt.Fprintf(&b, "💼 %s\n", strings.TrimSpace(j.Title))

	location := buildLocation(j)
	if location != "" {
		fmt.Fprintf(&b, "🏢 %s — 📍 %s\n", strings.TrimSpace(j.Company), location)
	} else {
		fmt.Fprintf(&b, "🏢 %s\n", strings.TrimSpace(j.Company))
	}

	desc := sum.Summarize(j.Description)
	if desc != "" {
		fmt.Fprintf(&b, "\n📝 توضیح: %s\n", desc)
	}

	fmt.Fprintf(&b, "\n🔗 مشاهده و درخواست: %s\n", j.ApplyLink)

	if d := datePart(j.PostedAt); d != "" {
		fmt.Fprintf(&b, "📅 تاریخ انتشار: %s\n", d)
	}

	return b.String()
}

func buildLocation(j jsearch.Job) string {
	parts := []string{}
	loc := strings.TrimSpace(strings.Join(nonEmpty(j.City, j.Country), ", "))
	if loc != "" {
		parts = append(parts, loc)
	}
	if j.IsRemote {
		parts = append(parts, "Remote")
	}
	return strings.Join(parts, " ")
}

func nonEmpty(vals ...string) []string {
	out := []string{}
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, strings.TrimSpace(v))
		}
	}
	return out
}

// datePart فقط بخش تاریخ (YYYY-MM-DD) را از رشته زمان ISO برمی‌گرداند.
func datePart(iso string) string {
	iso = strings.TrimSpace(iso)
	if len(iso) >= 10 {
		return iso[:10]
	}
	return ""
}
