// Package dedup تکراری‌های داخل یک اجرا را حذف می‌کند.
package dedup

import (
	"strings"

	"github.com/aghaie/job-finder/internal/core"
)

const similarityThreshold = 0.9

// Dedupe جاب‌های تکراری را حذف و اولین دیده‌شده را نگه می‌دارد.
// ابتدا بر اساس Fingerprint، سپس بر اساس تشابه Title+Company.
func Dedupe(jobs []core.Job) []core.Job {
	seen := make(map[string]struct{})
	var kept []core.Job
	var keptTokens []map[string]struct{}

	for _, j := range jobs {
		fp := j.Fingerprint()
		if _, ok := seen[fp]; ok {
			continue
		}
		tk := tokenize(j.Title + " " + j.Company)
		dup := false
		for _, kt := range keptTokens {
			if jaccard(tk, kt) >= similarityThreshold {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		seen[fp] = struct{}{}
		kept = append(kept, j)
		keptTokens = append(keptTokens, tk)
	}
	return kept
}

func tokenize(s string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, w := range strings.Fields(strings.ToLower(s)) {
		set[w] = struct{}{}
	}
	return set
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
