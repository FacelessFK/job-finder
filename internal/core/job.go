// Package core مدل دامنه و انواع مشترک است و به هیچ پکیج داخلی دیگری وابسته نیست.
package core

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"strings"
	"time"
)

// Job مدل واحد یک آگهی شغلی از هر منبع.
type Job struct {
	ID             string
	Title          string
	Company        string
	Location       string
	Remote         bool
	Relocation     bool
	EmploymentType string // FULLTIME | PARTTIME | CONTRACTOR | INTERN
	Seniority      string // junior | mid | senior | lead
	Description    string
	URL            string
	PostedAt       time.Time
	Source         string
}

// Filters معیارهای جست‌وجو و فیلترکردن.
type Filters struct {
	Countries         []string
	Keywords          []string
	ExcludeKeywords   []string
	EmploymentTypes   []string
	Seniority         []string
	RemoteOnly        bool
	RelocationOnly    bool
	PostedWithinHours int
	CompanyWhitelist  []string
	CompanyBlacklist  []string
}

// Decision نتیجه‌ی یک فیلتر.
type Decision struct {
	Publish bool
	Reason  string
}

// Fingerprint کلید یکتای جاب برای dedup و SeenStore.
// اولویت با URL نرمال‌شده؛ در نبودش هش Company+Title.
func (j Job) Fingerprint() string {
	if n := normalizeURL(j.URL); n != "" {
		return "url:" + n
	}
	key := strings.ToLower(strings.TrimSpace(j.Company)) + "|" +
		strings.ToLower(strings.TrimSpace(j.Title))
	sum := sha1.Sum([]byte(key))
	return "ct:" + hex.EncodeToString(sum[:])
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	path := strings.ToLower(strings.TrimRight(u.Path, "/"))
	return host + path
}
