# LinkedIn Job-Finder Telegram Service — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** یک سرویس Go که هر روز روی GitHub Actions اجرا می‌شود، جاب‌های جدید برنامه‌نویسی را از JSearch می‌گیرد و برای هر جاب جدید یک پیام فارسی به یک کانال تلگرام می‌فرستد.

**Architecture:** یک ابزار خط‌فرمان Go با پکیج‌های داخلی مجزا (config, jsearch, store, summarize, message, telegram) که `cmd/jobfinder/main.go` آن‌ها را به هم وصل می‌کند. ضدتکرار با فایل `data/seen_jobs.json` انجام می‌شود که پس از هر اجرا در مخزن commit می‌شود. اجرای زمان‌بندی‌شده با GitHub Actions.

**Tech Stack:** Go 1.22+، کتابخانه استاندارد (`net/http`, `encoding/json`, `testing`)، بدون وابستگی خارجی. JSearch روی RapidAPI. Telegram Bot API.

---

## File Structure

| فایل | مسئولیت |
|------|---------|
| `go.mod` | تعریف ماژول و نسخه Go |
| `internal/config/config.go` | خواندن متغیرهای محیطی + کلمات کلیدی و تنظیمات پیش‌فرض |
| `internal/jsearch/jsearch.go` | مدل جاب + کلاینت گرفتن جاب‌ها از JSearch |
| `internal/store/store.go` | خواندن/نوشتن `seen_jobs.json`، فیلتر جاب‌های جدید، هرس قدیمی‌ها |
| `internal/summarize/summarize.go` | رابط Summarizer + پیاده‌سازی ساده فاز ۱ |
| `internal/message/message.go` | ساخت متن پیام فارسی از یک جاب |
| `internal/telegram/telegram.go` | ارسال پیام به کانال با فاصله زمانی |
| `cmd/jobfinder/main.go` | هماهنگ‌کننده اصلی |
| `data/seen_jobs.json` | وضعیت آیدی جاب‌های ارسال‌شده |
| `.github/workflows/daily.yml` | زمان‌بندی روزانه + commit وضعیت |
| `.env.example` | نمونه متغیرهای محیطی |
| `README.md` | راهنمای راه‌اندازی |

**قرارداد نوع‌ها (types) که در چند تسک استفاده می‌شوند:**

```go
// internal/jsearch — مدل مشترک جاب
type Job struct {
    ID          string // job_id از JSearch
    Title       string // job_title
    Company     string // employer_name
    City        string // job_city
    Country     string // job_country
    IsRemote    bool   // job_is_remote
    Description string // job_description
    ApplyLink   string // job_apply_link
    PostedAt    string // job_posted_at_datetime_utc (رشته ISO یا خالی)
}
```

این نوع `jsearch.Job` در پکیج‌های store، summarize، message و main استفاده می‌شود.

---

## Task 1: راه‌اندازی ماژول Go

**Files:**
- Create: `go.mod`
- Create: `.gitignore`

- [ ] **Step 1: ساخت go.mod**

Create `go.mod`:

```
module github.com/aghaie/job-finder

go 1.22
```

- [ ] **Step 2: ساخت .gitignore**

Create `.gitignore`:

```
.env
jobfinder
/dist/
```

- [ ] **Step 3: تأیید کامپایل خالی**

Run: `go build ./...`
Expected: بدون خروجی و بدون خطا (کدی هنوز نیست).

- [ ] **Step 4: Commit**

```bash
git add go.mod .gitignore
git commit -m "chore: initialize Go module"
```

---

## Task 2: مدل Job و کلاینت JSearch

**Files:**
- Create: `internal/jsearch/jsearch.go`
- Test: `internal/jsearch/jsearch_test.go`

- [ ] **Step 1: نوشتن تست شکست‌خورده برای پارس پاسخ JSearch**

Create `internal/jsearch/jsearch_test.go`:

```go
package jsearch

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseSearchResponse(t *testing.T) {
	body := `{
	  "status": "OK",
	  "data": [
	    {
	      "job_id": "abc123",
	      "job_title": "Backend Developer",
	      "employer_name": "Acme GmbH",
	      "job_city": "Berlin",
	      "job_country": "DE",
	      "job_is_remote": true,
	      "job_description": "We build things.",
	      "job_apply_link": "https://example.com/apply",
	      "job_posted_at_datetime_utc": "2026-07-24T10:00:00.000Z"
	    }
	  ]
	}`
	jobs, err := parseSearchResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	if j.ID != "abc123" {
		t.Errorf("ID: got %q", j.ID)
	}
	if j.Title != "Backend Developer" {
		t.Errorf("Title: got %q", j.Title)
	}
	if j.Company != "Acme GmbH" {
		t.Errorf("Company: got %q", j.Company)
	}
	if !j.IsRemote {
		t.Errorf("IsRemote: expected true")
	}
	if j.ApplyLink != "https://example.com/apply" {
		t.Errorf("ApplyLink: got %q", j.ApplyLink)
	}
}

func TestFetchUsesQueryAndKey(t *testing.T) {
	var gotQuery, gotKey, gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		gotKey = r.Header.Get("X-RapidAPI-Key")
		gotHost = r.Header.Get("X-RapidAPI-Host")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"OK","data":[]}`))
	}))
	defer srv.Close()

	c := &Client{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := c.Fetch("golang developer", "today", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "golang developer" {
		t.Errorf("query: got %q", gotQuery)
	}
	if gotKey != "test-key" {
		t.Errorf("key header: got %q", gotKey)
	}
	if gotHost != "jsearch.p.rapidapi.com" {
		t.Errorf("host header: got %q", gotHost)
	}
}
```

- [ ] **Step 2: اجرای تست برای دیدن شکست**

Run: `go test ./internal/jsearch/`
Expected: FAIL — `undefined: parseSearchResponse` و `undefined: Client`.

- [ ] **Step 3: نوشتن پیاده‌سازی**

Create `internal/jsearch/jsearch.go`:

```go
// Package jsearch یک کلاینت کوچک برای JSearch API روی RapidAPI است.
package jsearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Job مدل نرمال‌شده یک آگهی شغلی است که در کل برنامه استفاده می‌شود.
type Job struct {
	ID          string
	Title       string
	Company     string
	City        string
	Country     string
	IsRemote    bool
	Description string
	ApplyLink   string
	PostedAt    string
}

// Client برای فراخوانی JSearch است.
type Client struct {
	APIKey     string
	BaseURL    string // پیش‌فرض: https://jsearch.p.rapidapi.com
	HTTPClient *http.Client
}

// NewClient یک کلاینت با مقادیر پیش‌فرض می‌سازد.
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    "https://jsearch.p.rapidapi.com",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type rawResponse struct {
	Status string    `json:"status"`
	Data   []rawJob  `json:"data"`
}

type rawJob struct {
	JobID       string `json:"job_id"`
	JobTitle    string `json:"job_title"`
	Employer    string `json:"employer_name"`
	City        string `json:"job_city"`
	Country     string `json:"job_country"`
	IsRemote    bool   `json:"job_is_remote"`
	Description string `json:"job_description"`
	ApplyLink   string `json:"job_apply_link"`
	PostedAt    string `json:"job_posted_at_datetime_utc"`
}

func parseSearchResponse(body []byte) ([]Job, error) {
	var raw rawResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse jsearch response: %w", err)
	}
	jobs := make([]Job, 0, len(raw.Data))
	for _, r := range raw.Data {
		jobs = append(jobs, Job{
			ID:          r.JobID,
			Title:       r.JobTitle,
			Company:     r.Employer,
			City:        r.City,
			Country:     r.Country,
			IsRemote:    r.IsRemote,
			Description: r.Description,
			ApplyLink:   r.ApplyLink,
			PostedAt:    r.PostedAt,
		})
	}
	return jobs, nil
}

// Fetch جاب‌ها را برای یک کلمه کلیدی می‌گیرد.
// datePosted یکی از: all, today, 3days, week, month.
func (c *Client) Fetch(query, datePosted string, page int) ([]Job, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://jsearch.p.rapidapi.com"
	}
	u, err := url.Parse(base + "/search")
	if err != nil {
		return nil, fmt.Errorf("bad base url: %w", err)
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("date_posted", datePosted)
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("num_pages", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-RapidAPI-Key", c.APIKey)
	req.Header.Set("X-RapidAPI-Host", "jsearch.p.rapidapi.com")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jsearch request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jsearch status %d: %s", resp.StatusCode, string(body))
	}
	return parseSearchResponse(body)
}
```

- [ ] **Step 4: اجرای تست برای دیدن موفقیت**

Run: `go test ./internal/jsearch/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/jsearch/
git commit -m "feat: add JSearch client and Job model"
```

---

## Task 3: انبار وضعیت (ضدتکرار)

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: نوشتن تست شکست‌خورده**

Create `internal/store/store_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"

	"github.com/aghaie/job-finder/internal/jsearch"
)

func TestFilterNewAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	jobs := []jsearch.Job{
		{ID: "a"}, {ID: "b"}, {ID: "a"}, // "a" دوبار در همین اجرا
	}
	fresh := s.FilterNew(jobs)
	if len(fresh) != 2 {
		t.Fatalf("expected 2 fresh jobs, got %d", len(fresh))
	}

	s.MarkSeen("a")
	s.MarkSeen("b")
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// بارگذاری دوباره: حالا a و b دیده‌شده‌اند
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	fresh2 := s2.FilterNew(jobs)
	if len(fresh2) != 0 {
		t.Fatalf("expected 0 fresh after reload, got %d", len(fresh2))
	}
}

func TestPruneKeepsMostRecent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")
	s, _ := Load(path)

	for _, id := range []string{"1", "2", "3", "4", "5"} {
		s.MarkSeen(id)
	}
	s.Prune(3) // فقط ۳ تای آخر بماند
	if got := len(s.ids); got != 3 {
		t.Fatalf("expected 3 ids after prune, got %d", got)
	}
	// جدیدترین‌ها باید بمانند
	if s.FilterNew([]jsearch.Job{{ID: "5"}}) != nil && len(s.FilterNew([]jsearch.Job{{ID: "5"}})) != 0 {
		t.Errorf("id 5 should still be seen")
	}
	if len(s.FilterNew([]jsearch.Job{{ID: "1"}})) != 1 {
		t.Errorf("id 1 should have been pruned")
	}
}
```

- [ ] **Step 2: اجرای تست برای دیدن شکست**

Run: `go test ./internal/store/`
Expected: FAIL — `undefined: Load`.

- [ ] **Step 3: نوشتن پیاده‌سازی**

Create `internal/store/store.go`:

```go
// Package store وضعیت جاب‌های دیده‌شده را برای جلوگیری از ارسال تکراری نگه می‌دارد.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aghaie/job-finder/internal/jsearch"
)

// Store مجموعه آیدی جاب‌های دیده‌شده را با حفظ ترتیب افزوده‌شدن نگه می‌دارد.
type Store struct {
	path  string
	ids   map[string]struct{}
	order []string // ترتیب افزوده‌شدن، برای هرس
}

type fileFormat struct {
	SeenIDs []string `json:"seen_ids"`
}

// Load وضعیت را از فایل می‌خواند. اگر فایل نباشد، وضعیت خالی برمی‌گرداند.
func Load(path string) (*Store, error) {
	s := &Store{
		path:  path,
		ids:   make(map[string]struct{}),
		order: nil,
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	for _, id := range f.SeenIDs {
		if _, ok := s.ids[id]; !ok {
			s.ids[id] = struct{}{}
			s.order = append(s.order, id)
		}
	}
	return s, nil
}

// FilterNew جاب‌هایی را برمی‌گرداند که نه در وضعیت ذخیره‌شده هستند و نه در همین
// فراخوانی تکراری‌اند. وضعیت را تغییر نمی‌دهد.
func (s *Store) FilterNew(jobs []jsearch.Job) []jsearch.Job {
	var fresh []jsearch.Job
	inBatch := make(map[string]struct{})
	for _, j := range jobs {
		if j.ID == "" {
			continue
		}
		if _, seen := s.ids[j.ID]; seen {
			continue
		}
		if _, dup := inBatch[j.ID]; dup {
			continue
		}
		inBatch[j.ID] = struct{}{}
		fresh = append(fresh, j)
	}
	return fresh
}

// MarkSeen یک آیدی را دیده‌شده علامت می‌زند.
func (s *Store) MarkSeen(id string) {
	if id == "" {
		return
	}
	if _, ok := s.ids[id]; ok {
		return
	}
	s.ids[id] = struct{}{}
	s.order = append(s.order, id)
}

// Prune فقط n آیدی آخر (جدیدترین‌ها) را نگه می‌دارد تا فایل بی‌نهایت بزرگ نشود.
func (s *Store) Prune(max int) {
	if max <= 0 || len(s.order) <= max {
		return
	}
	drop := s.order[:len(s.order)-max]
	for _, id := range drop {
		delete(s.ids, id)
	}
	s.order = s.order[len(s.order)-max:]
}

// Save وضعیت را روی دیسک می‌نویسد.
func (s *Store) Save() error {
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f := fileFormat{SeenIDs: s.order}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
```

- [ ] **Step 4: اجرای تست برای دیدن موفقیت**

Run: `go test ./internal/store/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add seen-jobs store with dedup and prune"
```

---

## Task 4: خلاصه‌ساز (فاز ۱ ساده)

**Files:**
- Create: `internal/summarize/summarize.go`
- Test: `internal/summarize/summarize_test.go`

- [ ] **Step 1: نوشتن تست شکست‌خورده**

Create `internal/summarize/summarize_test.go`:

```go
package summarize

import (
	"strings"
	"testing"
)

func TestSimpleSummarizerShortens(t *testing.T) {
	s := NewSimple(120)
	long := strings.Repeat("word ", 100) // خیلی طولانی
	out := s.Summarize(long)
	if len([]rune(out)) > 123 { // 120 + "..."
		t.Errorf("summary too long: %d runes", len([]rune(out)))
	}
	if !strings.HasSuffix(out, "...") {
		t.Errorf("expected ellipsis suffix, got %q", out)
	}
}

func TestSimpleSummarizerCollapsesWhitespace(t *testing.T) {
	s := NewSimple(200)
	out := s.Summarize("Hello\n\n   world\t\tfoo")
	if out != "Hello world foo" {
		t.Errorf("got %q", out)
	}
}

func TestSimpleSummarizerShortTextUnchanged(t *testing.T) {
	s := NewSimple(200)
	out := s.Summarize("Short one.")
	if out != "Short one." {
		t.Errorf("got %q", out)
	}
}
```

- [ ] **Step 2: اجرای تست برای دیدن شکست**

Run: `go test ./internal/summarize/`
Expected: FAIL — `undefined: NewSimple`.

- [ ] **Step 3: نوشتن پیاده‌سازی**

Create `internal/summarize/summarize.go`:

```go
// Package summarize توضیح جاب را برای نمایش در پیام کوتاه می‌کند.
// فاز ۱: کوتاه‌سازی ساده. فاز ۲: پیاده‌سازی مبتنی بر Claude API با همین رابط.
package summarize

import (
	"strings"
	"unicode"
)

// Summarizer رابطی است که در فاز ۲ می‌توان پیاده‌سازی هوشمند را جایگزین کرد.
type Summarizer interface {
	Summarize(description string) string
}

// Simple یک خلاصه‌ساز بدون AI است که فاصله‌ها را جمع و متن را کوتاه می‌کند.
type Simple struct {
	MaxRunes int
}

// NewSimple یک خلاصه‌ساز ساده با حداکثر تعداد کاراکتر می‌سازد.
func NewSimple(maxRunes int) *Simple {
	if maxRunes <= 0 {
		maxRunes = 300
	}
	return &Simple{MaxRunes: maxRunes}
}

// Summarize فاصله‌های اضافی را جمع می‌کند و در صورت طولانی‌بودن، متن را می‌برد.
func (s *Simple) Summarize(description string) string {
	fields := strings.FieldsFunc(description, func(r rune) bool {
		return unicode.IsSpace(r)
	})
	clean := strings.Join(fields, " ")
	runes := []rune(clean)
	if len(runes) <= s.MaxRunes {
		return clean
	}
	return string(runes[:s.MaxRunes]) + "..."
}
```

- [ ] **Step 4: اجرای تست برای دیدن موفقیت**

Run: `go test ./internal/summarize/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/summarize/
git commit -m "feat: add simple description summarizer with Summarizer interface"
```

---

## Task 5: سازنده پیام تلگرام

**Files:**
- Create: `internal/message/message.go`
- Test: `internal/message/message_test.go`

- [ ] **Step 1: نوشتن تست شکست‌خورده**

Create `internal/message/message_test.go`:

```go
package message

import (
	"strings"
	"testing"

	"github.com/aghaie/job-finder/internal/jsearch"
	"github.com/aghaie/job-finder/internal/summarize"
)

func TestBuild(t *testing.T) {
	j := jsearch.Job{
		Title:       "Backend Developer",
		Company:     "Acme GmbH",
		City:        "Berlin",
		Country:     "DE",
		IsRemote:    true,
		Description: "We build backend systems in Go.",
		ApplyLink:   "https://example.com/apply",
		PostedAt:    "2026-07-24T10:00:00.000Z",
	}
	sum := summarize.NewSimple(200)
	out := Build(j, sum)

	for _, want := range []string{
		"Backend Developer",
		"Acme GmbH",
		"Berlin",
		"https://example.com/apply",
		"2026-07-24",
		"Remote",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("message missing %q\n---\n%s", want, out)
		}
	}
}

func TestBuildHandlesMissingLocation(t *testing.T) {
	j := jsearch.Job{
		Title:     "Dev",
		Company:   "X",
		ApplyLink: "https://x.test",
	}
	out := Build(j, summarize.NewSimple(200))
	if !strings.Contains(out, "Dev") || !strings.Contains(out, "https://x.test") {
		t.Errorf("unexpected: %s", out)
	}
}
```

- [ ] **Step 2: اجرای تست برای دیدن شکست**

Run: `go test ./internal/message/`
Expected: FAIL — `undefined: Build`.

- [ ] **Step 3: نوشتن پیاده‌سازی**

Create `internal/message/message.go`:

```go
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
```

توجه: خروجی `buildLocation` وقتی هم شهر/کشور و هم Remote باشد، به شکل
`Berlin, DE Remote` می‌شود؛ تست فقط وجود زیررشته‌ها را چک می‌کند، پس مشکلی نیست.

- [ ] **Step 4: اجرای تست برای دیدن موفقیت**

Run: `go test ./internal/message/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/message/
git commit -m "feat: add Persian Telegram message builder"
```

---

## Task 6: فرستنده تلگرام

**Files:**
- Create: `internal/telegram/telegram.go`
- Test: `internal/telegram/telegram_test.go`

- [ ] **Step 1: نوشتن تست شکست‌خورده**

Create `internal/telegram/telegram_test.go`:

```go
package telegram

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendPostsToBotAPI(t *testing.T) {
	var gotPath, gotChatID, gotText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		r.ParseForm()
		gotChatID = r.FormValue("chat_id")
		gotText = r.FormValue("text")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := &Client{
		Token:      "123:ABC",
		ChatID:     "@mychannel",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}
	err := c.Send("hello world")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.Contains(gotPath, "/bot123:ABC/sendMessage") {
		t.Errorf("path: got %q", gotPath)
	}
	if gotChatID != "@mychannel" {
		t.Errorf("chat_id: got %q", gotChatID)
	}
	if gotText != "hello world" {
		t.Errorf("text: got %q", gotText)
	}
}

func TestSendReturnsErrorOnNotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer srv.Close()

	c := &Client{Token: "t", ChatID: "x", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if err := c.Send("hi"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

- [ ] **Step 2: اجرای تست برای دیدن شکست**

Run: `go test ./internal/telegram/`
Expected: FAIL — `undefined: Client`.

- [ ] **Step 3: نوشتن پیاده‌سازی**

Create `internal/telegram/telegram.go`:

```go
// Package telegram پیام‌ها را از طریق Telegram Bot API به یک کانال می‌فرستد.
package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client یک فرستنده پیام تلگرام است.
type Client struct {
	Token      string
	ChatID     string // مثل "@mychannel" یا آیدی عددی
	BaseURL    string // پیش‌فرض: https://api.telegram.org
	HTTPClient *http.Client
}

// NewClient یک کلاینت با مقادیر پیش‌فرض می‌سازد.
func NewClient(token, chatID string) *Client {
	return &Client{
		Token:      token,
		ChatID:     chatID,
		BaseURL:    "https://api.telegram.org",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

// Send یک پیام متنی به کانال می‌فرستد.
func (c *Client) Send(text string) error {
	base := c.BaseURL
	if base == "" {
		base = "https://api.telegram.org"
	}
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", base, c.Token)

	form := url.Values{}
	form.Set("chat_id", c.ChatID)
	form.Set("text", text)
	form.Set("disable_web_page_preview", "false")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var ar apiResponse
	_ = json.Unmarshal(body, &ar)
	if resp.StatusCode != http.StatusOK || !ar.OK {
		desc := ar.Description
		if desc == "" {
			desc = strings.TrimSpace(string(body))
		}
		return fmt.Errorf("telegram send failed (status %d): %s", resp.StatusCode, desc)
	}
	return nil
}
```

- [ ] **Step 4: اجرای تست برای دیدن موفقیت**

Run: `go test ./internal/telegram/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telegram/
git commit -m "feat: add Telegram channel sender"
```

---

## Task 7: پیکربندی

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: نوشتن تست شکست‌خورده**

Create `internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadReadsEnv(t *testing.T) {
	t.Setenv("RAPIDAPI_KEY", "k")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	t.Setenv("TELEGRAM_CHAT_ID", "@ch")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RapidAPIKey != "k" || cfg.TelegramToken != "tok" || cfg.TelegramChatID != "@ch" {
		t.Errorf("unexpected cfg: %+v", cfg)
	}
	if len(cfg.Keywords) == 0 {
		t.Error("expected default keywords")
	}
	if cfg.DatePosted == "" {
		t.Error("expected default date_posted")
	}
}

func TestLoadFailsWithoutRequired(t *testing.T) {
	t.Setenv("RAPIDAPI_KEY", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when required env missing")
	}
}
```

- [ ] **Step 2: اجرای تست برای دیدن شکست**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: Load`.

- [ ] **Step 3: نوشتن پیاده‌سازی**

Create `internal/config/config.go`:

```go
// Package config تنظیمات سرویس را از متغیرهای محیطی و مقادیر پیش‌فرض می‌خواند.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config تنظیمات کامل اجرا است.
type Config struct {
	RapidAPIKey    string
	TelegramToken  string
	TelegramChatID string

	Keywords     []string // کلمات کلیدی جست‌وجو
	DatePosted   string   // all|today|3days|week|month
	MaxPerRun    int      // سقف پیام در هر اجرا
	DelaySeconds int      // فاصله بین پیام‌ها
	SummaryRunes int      // حداکثر طول توضیح
	SeenPath     string   // مسیر فایل وضعیت
	MaxSeen      int      // حداکثر آیدی نگه‌داشته‌شده
}

// DefaultKeywords عنوان‌های رایج برنامه‌نویسی برای جست‌وجو.
var DefaultKeywords = []string{
	"software developer",
	"backend developer",
	"frontend developer",
	"full stack developer",
	"golang developer",
}

// Load تنظیمات را می‌خواند. سه متغیر توکن الزامی‌اند.
func Load() (*Config, error) {
	cfg := &Config{
		RapidAPIKey:    os.Getenv("RAPIDAPI_KEY"),
		TelegramToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID: os.Getenv("TELEGRAM_CHAT_ID"),
		Keywords:       DefaultKeywords,
		DatePosted:     getEnvDefault("DATE_POSTED", "today"),
		MaxPerRun:      getEnvInt("MAX_PER_RUN", 20),
		DelaySeconds:   getEnvInt("DELAY_SECONDS", 4),
		SummaryRunes:   getEnvInt("SUMMARY_RUNES", 300),
		SeenPath:       getEnvDefault("SEEN_PATH", "data/seen_jobs.json"),
		MaxSeen:        getEnvInt("MAX_SEEN", 5000),
	}

	if kw := os.Getenv("KEYWORDS"); kw != "" {
		parts := strings.Split(kw, ",")
		var cleaned []string
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				cleaned = append(cleaned, t)
			}
		}
		if len(cleaned) > 0 {
			cfg.Keywords = cleaned
		}
	}

	var missing []string
	if cfg.RapidAPIKey == "" {
		missing = append(missing, "RAPIDAPI_KEY")
	}
	if cfg.TelegramToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if cfg.TelegramChatID == "" {
		missing = append(missing, "TELEGRAM_CHAT_ID")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
```

- [ ] **Step 4: اجرای تست برای دیدن موفقیت**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config loader with env vars and defaults"
```

---

## Task 8: هماهنگ‌کننده اصلی

**Files:**
- Create: `cmd/jobfinder/main.go`

- [ ] **Step 1: نوشتن main**

Create `cmd/jobfinder/main.go`:

```go
// Command jobfinder جاب‌های جدید را از JSearch می‌گیرد و به کانال تلگرام می‌فرستد.
package main

import (
	"log"
	"time"

	"github.com/aghaie/job-finder/internal/config"
	"github.com/aghaie/job-finder/internal/jsearch"
	"github.com/aghaie/job-finder/internal/message"
	"github.com/aghaie/job-finder/internal/store"
	"github.com/aghaie/job-finder/internal/summarize"
	"github.com/aghaie/job-finder/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	js := jsearch.NewClient(cfg.RapidAPIKey)
	st, err := store.Load(cfg.SeenPath)
	if err != nil {
		log.Fatalf("store load error: %v", err)
	}
	tg := telegram.NewClient(cfg.TelegramToken, cfg.TelegramChatID)
	sum := summarize.NewSimple(cfg.SummaryRunes)

	// ۱) گرفتن جاب‌ها برای همه کلمات کلیدی
	var all []jsearch.Job
	for _, kw := range cfg.Keywords {
		jobs, err := js.Fetch(kw, cfg.DatePosted, 1)
		if err != nil {
			log.Printf("fetch %q failed: %v", kw, err)
			continue
		}
		log.Printf("fetched %d jobs for %q", len(jobs), kw)
		all = append(all, jobs...)
	}

	// ۲) فقط جاب‌های جدید
	fresh := st.FilterNew(all)
	log.Printf("%d new jobs after dedup", len(fresh))

	// ۳) ارسال با سقف و فاصله زمانی
	sent := 0
	for _, j := range fresh {
		if sent >= cfg.MaxPerRun {
			log.Printf("reached MaxPerRun=%d, stopping", cfg.MaxPerRun)
			break
		}
		text := message.Build(j, sum)
		if err := tg.Send(text); err != nil {
			log.Printf("send failed for %q: %v", j.ID, err)
			continue // آیدی را دیده‌شده علامت نمی‌زنیم تا دفعه بعد دوباره تلاش شود
		}
		st.MarkSeen(j.ID)
		sent++
		time.Sleep(time.Duration(cfg.DelaySeconds) * time.Second)
	}
	log.Printf("sent %d messages", sent)

	// ۴) هرس و ذخیره وضعیت
	st.Prune(cfg.MaxSeen)
	if err := st.Save(); err != nil {
		log.Fatalf("store save error: %v", err)
	}
	log.Printf("state saved to %s", cfg.SeenPath)
}
```

- [ ] **Step 2: تأیید کامپایل کل پروژه**

Run: `go build ./...`
Expected: بدون خطا.

- [ ] **Step 3: اجرای همه تست‌ها**

Run: `go test ./...`
Expected: همه PASS.

- [ ] **Step 4: بررسی vet**

Run: `go vet ./...`
Expected: بدون خطا.

- [ ] **Step 5: Commit**

```bash
git add cmd/
git commit -m "feat: add main orchestrator"
```

---

## Task 9: فایل وضعیت اولیه و نمونه محیط

**Files:**
- Create: `data/seen_jobs.json`
- Create: `.env.example`

- [ ] **Step 1: ساخت فایل وضعیت اولیه**

Create `data/seen_jobs.json`:

```json
{
  "seen_ids": []
}
```

- [ ] **Step 2: ساخت نمونه محیط**

Create `.env.example`:

```
# کلید RapidAPI برای JSearch
RAPIDAPI_KEY=your_rapidapi_key

# توکن بات تلگرام از BotFather
TELEGRAM_BOT_TOKEN=123456:ABC-your-bot-token

# آیدی کانال (مثل @mychannel یا آیدی عددی)
TELEGRAM_CHAT_ID=@mychannel

# اختیاری‌ها (مقادیر پیش‌فرض در کد هست)
# KEYWORDS=software developer,backend developer,golang developer
# DATE_POSTED=today
# MAX_PER_RUN=20
# DELAY_SECONDS=4
# SUMMARY_RUNES=300
```

- [ ] **Step 3: Commit**

```bash
git add data/seen_jobs.json .env.example
git commit -m "chore: add initial state file and env example"
```

---

## Task 10: اجرای محلی دستی (بررسی سلامت)

**Files:** بدون تغییر کد. این تسک فقط اجرای واقعی برای اطمینان است.

- [ ] **Step 1: ساخت فایل env محلی**

کاربر باید یک فایل `.env` واقعی بسازد (از روی `.env.example`) با کلیدها و توکن‌های واقعی.
این فایل نباید commit شود (در `.gitignore` هست).

- [ ] **Step 2: اجرای برنامه با بارگذاری env**

Run:
```bash
set -a && source .env && set +a && go run ./cmd/jobfinder
```
Expected: لاگ‌هایی مثل `fetched N jobs for ...`، `M new jobs after dedup`،
`sent K messages`، و در کانال تلگرام پیام‌ها ظاهر شوند.

- [ ] **Step 3: بررسی به‌روزرسانی وضعیت**

Run: `git status`
Expected: فایل `data/seen_jobs.json` تغییر کرده و آیدی‌های ارسال‌شده داخلش هست.

- [ ] **Step 4: اجرای دوباره برای تأیید ضدتکرار**

Run: `set -a && source .env && set +a && go run ./cmd/jobfinder`
Expected: `0 new jobs after dedup` (یا فقط جاب‌های تازه‌ی جدید) و پیام تکراری در کانال نباشد.

- [ ] **Step 5: Commit وضعیت اگر تغییر کرده**

```bash
git add data/seen_jobs.json
git commit -m "chore: update seen jobs after manual run"
```

---

## Task 11: GitHub Actions برای اجرای روزانه

**Files:**
- Create: `.github/workflows/daily.yml`

- [ ] **Step 1: نوشتن workflow**

Create `.github/workflows/daily.yml`:

```yaml
name: daily-job-finder

on:
  schedule:
    - cron: "0 6 * * *"   # هر روز ساعت ۰۶:۰۰ UTC
  workflow_dispatch: {}    # امکان اجرای دستی

permissions:
  contents: write          # برای commit فایل وضعیت

concurrency:
  group: job-finder
  cancel-in-progress: false

jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Run job finder
        env:
          RAPIDAPI_KEY: ${{ secrets.RAPIDAPI_KEY }}
          TELEGRAM_BOT_TOKEN: ${{ secrets.TELEGRAM_BOT_TOKEN }}
          TELEGRAM_CHAT_ID: ${{ secrets.TELEGRAM_CHAT_ID }}
        run: go run ./cmd/jobfinder

      - name: Commit updated state
        run: |
          git config user.name "job-finder-bot"
          git config user.email "bot@users.noreply.github.com"
          if ! git diff --quiet -- data/seen_jobs.json; then
            git add data/seen_jobs.json
            git commit -m "chore: update seen jobs [skip ci]"
            git push
          else
            echo "no state change"
          fi
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/daily.yml
git commit -m "ci: add daily GitHub Actions workflow"
```

- [ ] **Step 3: (دستی توسط کاربر) ثبت Secrets**

در مخزن گیت‌هاب، مسیر Settings → Secrets and variables → Actions، این سه مقدار اضافه شوند:
`RAPIDAPI_KEY`، `TELEGRAM_BOT_TOKEN`، `TELEGRAM_CHAT_ID`.

- [ ] **Step 4: (دستی) اجرای آزمایشی از تب Actions**

از تب Actions در گیت‌هاب، workflow به‌نام `daily-job-finder` را با `Run workflow` دستی اجرا کن
و لاگ‌ها و کانال تلگرام را بررسی کن.

---

## Task 12: مستندات (README)

**Files:**
- Create: `README.md`

- [ ] **Step 1: نوشتن README**

Create `README.md`:

````markdown
# Job Finder → Telegram

هر روز جاب‌های جدید برنامه‌نویسی را از JSearch می‌گیرد و به یک کانال تلگرام می‌فرستد.

## پیش‌نیازها

1. **کلید RapidAPI (JSearch):** در https://rapidapi.com ثبت‌نام کن، به JSearch مشترک شو
   (پلن رایگان برای شروع کافی است) و کلید `X-RapidAPI-Key` را بردار.
2. **بات تلگرام:** در تلگرام به `@BotFather` پیام بده، دستور `/newbot` را بزن،
   نام و یوزرنیم بده و توکن را بردار.
3. **کانال تلگرام:** یک کانال بساز، بات را به‌عنوان ادمین اضافه کن،
   و آیدی کانال (`@yourchannel`) را بردار.

## اجرای محلی

```bash
cp .env.example .env
# .env را با مقادیر واقعی پر کن
set -a && source .env && set +a
go run ./cmd/jobfinder
```

## اجرای خودکار (GitHub Actions)

سه مقدار زیر را در Settings → Secrets and variables → Actions مخزن ثبت کن:

- `RAPIDAPI_KEY`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`

workflow هر روز ساعت ۰۶:۰۰ UTC اجرا می‌شود و می‌توان از تب Actions دستی هم اجرایش کرد.

## تنظیمات (متغیرهای محیطی اختیاری)

| متغیر | پیش‌فرض | توضیح |
|-------|---------|-------|
| `KEYWORDS` | چند عنوان رایج | کلمات کلیدی جست‌وجو، جداشده با کاما |
| `DATE_POSTED` | `today` | `all` \| `today` \| `3days` \| `week` \| `month` |
| `MAX_PER_RUN` | `20` | سقف پیام در هر اجرا |
| `DELAY_SECONDS` | `4` | فاصله بین پیام‌ها |
| `SUMMARY_RUNES` | `300` | حداکثر طول توضیح |

## محدودیت‌ها

- پلن رایگان JSearch درخواست محدودی دارد؛ برای حجم بالاتر پلن پولی لازم است.
- خلاصه فارسی در فاز بعدی با Claude API اضافه می‌شود.
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README with setup guide"
```

---

## Self-Review Notes

- **Spec coverage:** همه اجزای سند طراحی (config, jsearch, store, summarize, message,
  telegram, main, workflow، فایل وضعیت، README) به تسک نگاشت شده‌اند.
- **Type consistency:** نوع `jsearch.Job` یک‌بار تعریف و همه‌جا استفاده می‌شود.
  امضاها: `Client.Fetch(query, datePosted string, page int)`, `store.Load(path)`,
  `Store.FilterNew([]jsearch.Job)`, `Store.MarkSeen(string)`, `Store.Prune(int)`,
  `Store.Save()`, `summarize.NewSimple(int)`, `Summarizer.Summarize(string)`,
  `message.Build(jsearch.Job, summarize.Summarizer)`, `telegram.NewClient(token, chatID)`,
  `Client.Send(text)` — در main هماهنگ‌اند.
- **امنیت:** توکن‌ها فقط از env و Secrets خوانده می‌شوند؛ `.env` در `.gitignore` است.
