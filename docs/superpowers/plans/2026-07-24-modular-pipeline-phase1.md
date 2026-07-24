# Modular Job Pipeline — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** ریفکتور پروژه به یک Pipeline ماژولار و چندمنبعی: `Providers → Normalize → Merge → Dedup → Filter(rule-based) → Publish(Telegram)`، با interfaceهای تمیز که افزودن Provider/Filter/Store/Publisher جدید را بدون تغییر بخش‌های دیگر ممکن کند. صفر وابستگی خارجی.

**Architecture:** Clean Architecture. `pipeline` فقط به interfaceها و `core` وابسته است. هر منبع یک `Provider`، هر فیلتر یک `Filter`، ذخیره‌سازی یک `SeenStore`، انتشار یک `Publisher`. وصل‌کردن قطعات فقط در `main`.

**Tech Stack:** Go 1.22+، فقط stdlib (`net/http`, `encoding/json`, `log/slog`, `context`, `time`). ماژول: `github.com/aghaie/job-finder`.

**اجرای محلی:** از `~/sdk/go/bin/go` با `GOTOOLCHAIN=local` استفاده کن (نسخه‌ی سیستمی 1.13 قدیمی است).

---

## قراردادهای سراسری (Types & Signatures)

این‌ها در چند تسک استفاده می‌شوند؛ نام‌ها باید دقیقاً همین بمانند:

```go
// core
type Job struct {
    ID, Title, Company, Location   string
    Remote, Relocation             bool
    EmploymentType, Seniority      string
    Description, URL               string
    PostedAt                       time.Time
    Source                         string
}
func (j Job) Fingerprint() string

type Filters struct {
    Countries, Keywords, ExcludeKeywords, EmploymentTypes, Seniority []string
    RemoteOnly, RelocationOnly bool
    PostedWithinHours          int
    CompanyWhitelist, CompanyBlacklist []string
}
type Decision struct { Publish bool; Reason string }

type Secrets struct {
    RapidAPIKey, LinkedInKey, LinkedInHost, LinkedInPath, TelegramToken, TelegramChatID string
}
type Config struct {
    Providers           []string
    Filters             Filters
    MaxPerRun           int
    DelaySeconds        int
    AllowInternship     bool
    MinDescriptionRunes int
    SeenPath            string
    MaxSeen             int
    Secrets             Secrets
}

// providers
type Provider interface { Name() string; SearchJobs(ctx, core.Filters) ([]core.Job, error) }
type Factory  func(cfg core.Config) (Provider, error)
func Register(name string, f Factory)
func Build(names []string, cfg core.Config) ([]Provider, error)

// filter
type Filter interface { Name() string; Evaluate(ctx, core.Job) (core.Decision, error) }
type Chain struct{...}; func NewChain(log, ...Filter) *Chain; func (c *Chain) Allow(ctx, core.Job) bool
func BuildRuleChain(log *slog.Logger, cfg core.Config) *Chain

// store
type SeenStore interface { IsSeen(fp string) bool; MarkSeen(fp string); Save(ctx) error }
func NewFileStore(path string, maxSeen int) (*FileStore, error)

// publisher
type Publisher interface { Publish(ctx, core.Job) error }
func telegram.New(cfg core.Config) (*Client, error)

// pipeline
func New(provs []providers.Provider, chain *filter.Chain, st store.SeenStore, pub publisher.Publisher, log *slog.Logger, cfg core.Config) *Pipeline
func (p *Pipeline) Run(ctx) (Stats, error)
```

---

## Task 1: پاک‌سازی کد قدیمی و مدل دامنه `core`

کد قدیمی (نسخه‌ی ساده) ریفکتور می‌شود؛ پکیج‌های قدیمی حذف و منطق‌شان در ساختار جدید بازنویسی می‌شود.

**Files:**
- Delete: `internal/config/`, `internal/jsearch/`, `internal/message/`, `internal/store/`, `internal/summarize/`, `internal/telegram/`, `cmd/jobfinder/main.go`
- Create: `internal/core/job.go`, `internal/core/config.go`
- Test: `internal/core/job_test.go`

- [ ] **Step 1: حذف پکیج‌های قدیمی**

```bash
git rm -r internal/config internal/jsearch internal/message internal/store internal/summarize internal/telegram cmd/jobfinder/main.go
```

- [ ] **Step 2: نوشتن تست `core`**

Create `internal/core/job_test.go`:

```go
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
```

- [ ] **Step 3: نوشتن `core/job.go`**

Create `internal/core/job.go`:

```go
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
```

- [ ] **Step 4: نوشتن `core/config.go`**

Create `internal/core/config.go`:

```go
package core

// Secrets رازهایی که فقط از متغیرهای محیطی می‌آیند.
type Secrets struct {
	RapidAPIKey    string // JSearch
	LinkedInKey    string
	LinkedInHost   string
	LinkedInPath   string
	TelegramToken  string
	TelegramChatID string
}

// Config تنظیمات کامل اجرا (فیلترها از فایل، رازها از env).
type Config struct {
	Providers           []string
	Filters             Filters
	MaxPerRun           int
	DelaySeconds        int
	AllowInternship     bool
	MinDescriptionRunes int
	SeenPath            string
	MaxSeen             int
	Secrets             Secrets
}
```

- [ ] **Step 5: بیلد go.mod و تست**

```bash
export PATH="$HOME/sdk/go/bin:$PATH"; export GOTOOLCHAIN=local
go test ./internal/core/
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "refactor: remove old packages, add core domain model"
```

---

## Task 2: کمکی‌های `retry` و `ratelimit`

**Files:**
- Create: `internal/retry/retry.go`, `internal/retry/retry_test.go`
- Create: `internal/ratelimit/ratelimit.go`, `internal/ratelimit/ratelimit_test.go`

- [ ] **Step 1: تست retry**

Create `internal/retry/retry_test.go`:

```go
package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSucceedsAfterRetries(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 3, time.Millisecond, func(error) bool { return true }, func() error {
		calls++
		if calls < 3 {
			return errors.New("temp")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoStopsOnNonRetryable(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 5, time.Millisecond, func(error) bool { return false }, func() error {
		calls++
		return errors.New("fatal")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}
```

- [ ] **Step 2: پیاده‌سازی retry**

Create `internal/retry/retry.go`:

```go
// Package retry تلاش مجدد با backoff نمایی برای خطاهای موقت.
package retry

import (
	"context"
	"time"
)

// Do تابع fn را تا attempts بار اجرا می‌کند. اگر isRetryable(err) نادرست باشد بلافاصله
// برمی‌گردد. بین تلاش‌ها با backoff نمایی صبر می‌کند.
func Do(ctx context.Context, attempts int, base time.Duration, isRetryable func(error) bool, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var err error
	delay := base
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if isRetryable != nil && !isRetryable(err) {
			return err
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
	}
	return err
}
```

- [ ] **Step 3: تست ratelimit**

Create `internal/ratelimit/ratelimit_test.go`:

```go
package ratelimit

import (
	"context"
	"testing"
)

func TestZeroIntervalPasses(t *testing.T) {
	l := New(0)
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCanceledContext(t *testing.T) {
	l := New(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(ctx); err == nil {
		t.Fatal("expected context error")
	}
}
```

- [ ] **Step 4: پیاده‌سازی ratelimit**

Create `internal/ratelimit/ratelimit.go`:

```go
// Package ratelimit یک محدودکننده‌ی نرخِ ساده (حداقل فاصله بین فراخوانی‌ها).
package ratelimit

import (
	"context"
	"time"
)

// Limiter حداقل فاصله بین فراخوانی‌ها را تضمین می‌کند. امن برای استفاده‌ی هم‌زمان.
type Limiter struct {
	interval time.Duration
	tokens   chan struct{}
	last     time.Time
}

// New یک Limiter با فاصله‌ی مشخص می‌سازد.
func New(interval time.Duration) *Limiter {
	l := &Limiter{interval: interval, tokens: make(chan struct{}, 1)}
	l.tokens <- struct{}{}
	return l
}

// Wait تا زمان مجاز برای فراخوانی بعدی صبر می‌کند.
func (l *Limiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.tokens:
	}
	defer func() { l.tokens <- struct{}{} }()

	if l.interval > 0 {
		if wait := l.interval - time.Since(l.last); wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	l.last = time.Now()
	return nil
}
```

- [ ] **Step 5: تست و Commit**

```bash
go test ./internal/retry/ ./internal/ratelimit/
git add internal/retry internal/ratelimit && git commit -m "feat: add retry and ratelimit helpers"
```
Expected: PASS.

---

## Task 3: `normalize`

**Files:**
- Create: `internal/normalize/normalize.go`, `internal/normalize/normalize_test.go`

- [ ] **Step 1: تست**

Create `internal/normalize/normalize_test.go`:

```go
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
```

- [ ] **Step 2: پیاده‌سازی**

Create `internal/normalize/normalize.go`:

```go
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
	case strings.Contains(l, "contract"), strings.Contains(l, "contractor"), strings.Contains(l, "freelance"):
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
```

- [ ] **Step 3: تست و Commit**

```bash
go test ./internal/normalize/
git add internal/normalize && git commit -m "feat: add normalize package"
```
Expected: PASS.

---

## Task 4: `dedup`

**Files:**
- Create: `internal/dedup/dedup.go`, `internal/dedup/dedup_test.go`

- [ ] **Step 1: تست**

Create `internal/dedup/dedup_test.go`:

```go
package dedup

import (
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

func TestDedupeByURL(t *testing.T) {
	jobs := []core.Job{
		{Title: "A", Company: "X", URL: "https://ex.com/1"},
		{Title: "B", Company: "Y", URL: "https://www.ex.com/1/"},
		{Title: "C", Company: "Z", URL: "https://ex.com/2"},
	}
	out := Dedupe(jobs)
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d", len(out))
	}
}

func TestDedupeBySimilarTitleCompany(t *testing.T) {
	jobs := []core.Job{
		{Title: "Senior Backend Engineer", Company: "Stripe"},
		{Title: "Senior Backend Engineer", Company: "Stripe"},
		{Title: "Frontend Developer", Company: "Vercel"},
	}
	out := Dedupe(jobs)
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d", len(out))
	}
}
```

- [ ] **Step 2: پیاده‌سازی**

Create `internal/dedup/dedup.go`:

```go
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
```

- [ ] **Step 3: تست و Commit**

```bash
go test ./internal/dedup/
git add internal/dedup && git commit -m "feat: add in-batch dedup"
```
Expected: PASS.

---

## Task 5: `store` (interface + file)

**Files:**
- Create: `internal/store/store.go`, `internal/store/file.go`, `internal/store/file_test.go`

- [ ] **Step 1: تست**

Create `internal/store/file_test.go`:

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileStoreSeenAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.json")
	s, err := NewFileStore(path, 100)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if s.IsSeen("a") {
		t.Fatal("should not be seen yet")
	}
	s.MarkSeen("a")
	if !s.IsSeen("a") {
		t.Fatal("should be seen")
	}
	if err := s.Save(context.Background()); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2, _ := NewFileStore(path, 100)
	if !s2.IsSeen("a") {
		t.Fatal("should persist across reload")
	}
}

func TestFileStorePrune(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.json")
	s, _ := NewFileStore(path, 3)
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		s.MarkSeen(id)
	}
	if err := s.Save(context.Background()); err != nil {
		t.Fatalf("save: %v", err)
	}
	s2, _ := NewFileStore(path, 3)
	if s2.IsSeen("1") {
		t.Error("oldest should be pruned")
	}
	if !s2.IsSeen("5") {
		t.Error("newest should remain")
	}
}
```

- [ ] **Step 2: پیاده‌سازی interface**

Create `internal/store/store.go`:

```go
// Package store وضعیت جاب‌های منتشرشده را برای جلوگیری از ارسال تکراری نگه می‌دارد.
package store

import "context"

// SeenStore رابط ذخیره‌سازی Fingerprintهای دیده‌شده.
type SeenStore interface {
	IsSeen(fingerprint string) bool
	MarkSeen(fingerprint string)
	Save(ctx context.Context) error
}
```

- [ ] **Step 3: پیاده‌سازی فایلی**

Create `internal/store/file.go`:

```go
package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// FileStore پیاده‌سازی SeenStore روی یک فایل JSON. فقط وقتی تغییری باشد می‌نویسد.
type FileStore struct {
	path    string
	maxSeen int
	ids     map[string]struct{}
	order   []string
	dirty   bool
}

type fileFormat struct {
	Seen []string `json:"seen"`
}

// NewFileStore وضعیت را از فایل می‌خواند (اگر نبود، خالی).
func NewFileStore(path string, maxSeen int) (*FileStore, error) {
	s := &FileStore{path: path, maxSeen: maxSeen, ids: make(map[string]struct{})}
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
	for _, id := range f.Seen {
		if _, ok := s.ids[id]; !ok {
			s.ids[id] = struct{}{}
			s.order = append(s.order, id)
		}
	}
	return s, nil
}

func (s *FileStore) IsSeen(fp string) bool {
	_, ok := s.ids[fp]
	return ok
}

func (s *FileStore) MarkSeen(fp string) {
	if fp == "" {
		return
	}
	if _, ok := s.ids[fp]; ok {
		return
	}
	s.ids[fp] = struct{}{}
	s.order = append(s.order, fp)
	s.dirty = true
}

// Save فقط در صورت تغییر می‌نویسد و آیدی‌های قدیمی را هرس می‌کند.
func (s *FileStore) Save(ctx context.Context) error {
	if !s.dirty {
		return nil
	}
	s.prune()
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(fileFormat{Seen: s.order}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

func (s *FileStore) prune() {
	if s.maxSeen <= 0 || len(s.order) <= s.maxSeen {
		return
	}
	drop := s.order[:len(s.order)-s.maxSeen]
	for _, id := range drop {
		delete(s.ids, id)
	}
	s.order = s.order[len(s.order)-s.maxSeen:]
}
```

- [ ] **Step 4: تست و Commit**

```bash
go test ./internal/store/
git add internal/store && git commit -m "feat: add SeenStore interface and file implementation"
```
Expected: PASS.

---

## Task 6: `filter` (interface + chain + rule filters)

**Files:**
- Create: `internal/filter/filter.go`, `internal/filter/rules.go`, `internal/filter/filter_test.go`

- [ ] **Step 1: تست**

Create `internal/filter/filter_test.go`:

```go
package filter

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aghaie/job-finder/internal/core"
)

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func baseCfg() core.Config {
	return core.Config{
		MinDescriptionRunes: 10,
		Filters: core.Filters{
			Keywords: []string{"engineer"},
		},
	}
}

func TestChainRejectsInternship(t *testing.T) {
	c := BuildRuleChain(quietLog(), baseCfg())
	j := core.Job{Title: "Engineer", EmploymentType: "INTERN", Description: "a long enough description here"}
	if c.Allow(context.Background(), j) {
		t.Error("internship should be rejected by default")
	}
}

func TestChainRejectsMissingKeyword(t *testing.T) {
	c := BuildRuleChain(quietLog(), baseCfg())
	j := core.Job{Title: "Sales Manager", Description: "a long enough description here"}
	if c.Allow(context.Background(), j) {
		t.Error("job without keyword should be rejected")
	}
}

func TestChainRejectsShortDescription(t *testing.T) {
	c := BuildRuleChain(quietLog(), baseCfg())
	j := core.Job{Title: "Engineer", Description: "short"}
	if c.Allow(context.Background(), j) {
		t.Error("too-short description should be rejected")
	}
}

func TestChainRemoteOnly(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.RemoteOnly = true
	c := BuildRuleChain(quietLog(), cfg)
	remote := core.Job{Title: "Engineer", Remote: true, Description: "a long enough description here"}
	onsite := core.Job{Title: "Engineer", Remote: false, Description: "a long enough description here"}
	if !c.Allow(context.Background(), remote) {
		t.Error("remote job should pass")
	}
	if c.Allow(context.Background(), onsite) {
		t.Error("non-remote job should be rejected under remoteOnly")
	}
}

func TestChainDefaultRequiresRemoteOrRelocation(t *testing.T) {
	c := BuildRuleChain(quietLog(), baseCfg())
	neither := core.Job{Title: "Engineer", Description: "a long enough description here"}
	if c.Allow(context.Background(), neither) {
		t.Error("job that is neither remote nor relocation should be rejected by default")
	}
	reloc := core.Job{Title: "Engineer", Relocation: true, Description: "a long enough description here"}
	if !c.Allow(context.Background(), reloc) {
		t.Error("relocation job should pass default value filter")
	}
}

func TestChainFreshness(t *testing.T) {
	cfg := baseCfg()
	cfg.Filters.PostedWithinHours = 24
	c := BuildRuleChain(quietLog(), cfg)
	old := core.Job{Title: "Engineer", Remote: true, Description: "a long enough description here", PostedAt: time.Now().Add(-48 * time.Hour)}
	if c.Allow(context.Background(), old) {
		t.Error("old job should be rejected")
	}
	fresh := core.Job{Title: "Engineer", Remote: true, Description: "a long enough description here", PostedAt: time.Now().Add(-2 * time.Hour)}
	if !c.Allow(context.Background(), fresh) {
		t.Error("fresh job should pass")
	}
}
```

- [ ] **Step 2: پیاده‌سازی interface و chain**

Create `internal/filter/filter.go`:

```go
// Package filter زنجیره‌ی فیلترهاست؛ جاب باید از همه رد شود تا منتشر گردد.
package filter

import (
	"context"
	"log/slog"

	"github.com/aghaie/job-finder/internal/core"
)

// Filter یک قاعده‌ی فیلترکردن (قانون‌محور یا AI).
type Filter interface {
	Name() string
	Evaluate(ctx context.Context, job core.Job) (core.Decision, error)
}

// Chain چند فیلتر را پشت سر هم اجرا می‌کند.
type Chain struct {
	filters []Filter
	log     *slog.Logger
}

// NewChain یک زنجیره می‌سازد.
func NewChain(log *slog.Logger, filters ...Filter) *Chain {
	return &Chain{filters: filters, log: log}
}

// Allow اگر جاب از همه‌ی فیلترها رد شود true برمی‌گرداند؛ در غیر این‌صورت دلیل را لاگ می‌کند.
func (c *Chain) Allow(ctx context.Context, job core.Job) bool {
	for _, f := range c.filters {
		d, err := f.Evaluate(ctx, job)
		if err != nil {
			c.log.Warn("filter error", "filter", f.Name(), "job", job.Title, "err", err)
			return false
		}
		if !d.Publish {
			c.log.Info("job rejected", "filter", f.Name(), "job", job.Title, "company", job.Company, "reason", d.Reason)
			return false
		}
	}
	return true
}
```

- [ ] **Step 3: پیاده‌سازی فیلترهای قانون‌محور**

Create `internal/filter/rules.go`:

```go
package filter

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/aghaie/job-finder/internal/core"
)

// BuildRuleChain زنجیره‌ی فیلترهای قانون‌محور را از روی config می‌سازد.
func BuildRuleChain(log *slog.Logger, cfg core.Config) *Chain {
	f := cfg.Filters
	return NewChain(log,
		employmentFilter{allowInternship: cfg.AllowInternship, allowed: upperAll(f.EmploymentTypes)},
		keywordFilter{keywords: lowerAll(f.Keywords), exclude: lowerAll(f.ExcludeKeywords)},
		companyFilter{whitelist: lowerAll(f.CompanyWhitelist), blacklist: lowerAll(f.CompanyBlacklist)},
		freshnessFilter{withinHours: f.PostedWithinHours},
		valueFilter{remoteOnly: f.RemoteOnly, relocationOnly: f.RelocationOnly},
		spamFilter{minRunes: cfg.MinDescriptionRunes},
	)
}

// --- Employment ---

type employmentFilter struct {
	allowInternship bool
	allowed         []string
}

func (e employmentFilter) Name() string { return "employment" }
func (e employmentFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	if j.EmploymentType == "INTERN" && !e.allowInternship {
		return reject("internship not allowed"), nil
	}
	if len(e.allowed) > 0 && j.EmploymentType != "" && !contains(e.allowed, j.EmploymentType) {
		return reject("employment type not in allowed list"), nil
	}
	return pass(), nil
}

// --- Keyword ---

type keywordFilter struct {
	keywords []string
	exclude  []string
}

func (k keywordFilter) Name() string { return "keyword" }
func (k keywordFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	hay := strings.ToLower(j.Title + " " + j.Description)
	for _, ex := range k.exclude {
		if strings.Contains(hay, ex) {
			return reject("matched exclude keyword: " + ex), nil
		}
	}
	if len(k.keywords) > 0 {
		for _, kw := range k.keywords {
			if strings.Contains(hay, kw) {
				return pass(), nil
			}
		}
		return reject("no required keyword matched"), nil
	}
	return pass(), nil
}

// --- Company ---

type companyFilter struct {
	whitelist []string
	blacklist []string
}

func (c companyFilter) Name() string { return "company" }
func (c companyFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	comp := strings.ToLower(j.Company)
	for _, b := range c.blacklist {
		if b != "" && strings.Contains(comp, b) {
			return reject("company blacklisted"), nil
		}
	}
	if len(c.whitelist) > 0 {
		for _, w := range c.whitelist {
			if w != "" && strings.Contains(comp, w) {
				return pass(), nil
			}
		}
		return reject("company not in whitelist"), nil
	}
	return pass(), nil
}

// --- Freshness ---

type freshnessFilter struct {
	withinHours int
}

func (fr freshnessFilter) Name() string { return "freshness" }
func (fr freshnessFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	if fr.withinHours <= 0 || j.PostedAt.IsZero() {
		return pass(), nil
	}
	if time.Since(j.PostedAt) > time.Duration(fr.withinHours)*time.Hour {
		return reject("older than freshness window"), nil
	}
	return pass(), nil
}

// --- Value (Remote OR Relocation) ---

type valueFilter struct {
	remoteOnly     bool
	relocationOnly bool
}

func (v valueFilter) Name() string { return "value" }
func (v valueFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	switch {
	case v.remoteOnly && v.relocationOnly:
		if j.Remote && j.Relocation {
			return pass(), nil
		}
		return reject("requires both remote and relocation"), nil
	case v.remoteOnly:
		if j.Remote {
			return pass(), nil
		}
		return reject("not remote"), nil
	case v.relocationOnly:
		if j.Relocation {
			return pass(), nil
		}
		return reject("no relocation"), nil
	default:
		if j.Remote || j.Relocation {
			return pass(), nil
		}
		return reject("neither remote nor relocation"), nil
	}
}

// --- Spam / quality ---

type spamFilter struct {
	minRunes int
}

func (s spamFilter) Name() string { return "spam" }
func (s spamFilter) Evaluate(_ context.Context, j core.Job) (core.Decision, error) {
	if s.minRunes > 0 && len([]rune(strings.TrimSpace(j.Description))) < s.minRunes {
		return reject("description too short"), nil
	}
	return pass(), nil
}

// --- helpers ---

func pass() core.Decision            { return core.Decision{Publish: true} }
func reject(r string) core.Decision  { return core.Decision{Publish: false, Reason: r} }
func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
func lowerAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToLower(strings.TrimSpace(s)))
	}
	return out
}
func upperAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToUpper(strings.TrimSpace(s)))
	}
	return out
}
```

- [ ] **Step 4: تست و Commit**

```bash
go test ./internal/filter/
git add internal/filter && git commit -m "feat: add filter chain with rule-based filters"
```
Expected: PASS.

---

## Task 7: `providers` (interface + registry)

**Files:**
- Create: `internal/providers/providers.go`, `internal/providers/providers_test.go`

- [ ] **Step 1: تست**

Create `internal/providers/providers_test.go`:

```go
package providers

import (
	"context"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

type fakeProvider struct{ name string }

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) SearchJobs(context.Context, core.Filters) ([]core.Job, error) {
	return nil, nil
}

func TestRegisterAndBuild(t *testing.T) {
	Register("fake", func(core.Config) (Provider, error) { return fakeProvider{"fake"}, nil })
	ps, err := Build([]string{"fake"}, core.Config{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(ps) != 1 || ps[0].Name() != "fake" {
		t.Fatalf("unexpected: %+v", ps)
	}
}

func TestBuildUnknown(t *testing.T) {
	if _, err := Build([]string{"nope"}, core.Config{}); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
```

- [ ] **Step 2: پیاده‌سازی**

Create `internal/providers/providers.go`:

```go
// Package providers رابط منابع جاب و رجیستری آن‌ها را تعریف می‌کند.
package providers

import (
	"context"
	"fmt"

	"github.com/aghaie/job-finder/internal/core"
)

// Provider یک منبع جاب.
type Provider interface {
	Name() string
	SearchJobs(ctx context.Context, f core.Filters) ([]core.Job, error)
}

// Factory یک Provider را از روی config می‌سازد.
type Factory func(cfg core.Config) (Provider, error)

var registry = map[string]Factory{}

// Register یک Provider را با نامش ثبت می‌کند (معمولاً در init هر پکیج).
func Register(name string, f Factory) {
	registry[name] = f
}

// Build فقط Providerهای نام‌برده‌شده را می‌سازد.
func Build(names []string, cfg core.Config) ([]Provider, error) {
	out := make([]Provider, 0, len(names))
	for _, n := range names {
		f, ok := registry[n]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", n)
		}
		p, err := f(cfg)
		if err != nil {
			return nil, fmt.Errorf("build provider %q: %w", n, err)
		}
		out = append(out, p)
	}
	return out, nil
}
```

- [ ] **Step 3: تست و Commit**

```bash
go test ./internal/providers/
git add internal/providers/providers.go internal/providers/providers_test.go && git commit -m "feat: add provider interface and registry"
```
Expected: PASS.

---

## Task 8: Provider — JSearch

**Files:**
- Create: `internal/providers/jsearch/jsearch.go`, `internal/providers/jsearch/jsearch_test.go`

- [ ] **Step 1: تست**

Create `internal/providers/jsearch/jsearch_test.go`:

```go
package jsearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

func TestSearchMapsJobs(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-RapidAPI-Key")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"OK","data":[{
			"job_id":"j1","job_title":"Backend Engineer","employer_name":"Acme",
			"job_city":"Berlin","job_country":"DE","job_is_remote":true,
			"job_description":"desc","job_apply_link":"https://acme.test/1",
			"job_employment_type":"FULLTIME","job_posted_at_datetime_utc":"2026-07-24T10:00:00Z"}]}`))
	}))
	defer srv.Close()

	p := &Provider{key: "k", baseURL: srv.URL, client: srv.Client()}
	jobs, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"backend"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	if j.Title != "Backend Engineer" || j.Company != "Acme" || j.Source != "jsearch" {
		t.Errorf("bad mapping: %+v", j)
	}
	if !j.Remote || j.URL != "https://acme.test/1" {
		t.Errorf("bad fields: %+v", j)
	}
	if gotKey != "k" {
		t.Errorf("key header: %q", gotKey)
	}
}
```

- [ ] **Step 2: پیاده‌سازی**

Create `internal/providers/jsearch/jsearch.go`:

```go
// Package jsearch منبع JSearch (RapidAPI) را پیاده می‌کند.
package jsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/retry"
)

func init() {
	providers.Register("jsearch", New)
}

// Provider منبع JSearch.
type Provider struct {
	key     string
	baseURL string
	client  *http.Client
}

// New یک Provider از روی config می‌سازد.
func New(cfg core.Config) (providers.Provider, error) {
	if cfg.Secrets.RapidAPIKey == "" {
		return nil, fmt.Errorf("RAPIDAPI_KEY is required for jsearch")
	}
	return &Provider{
		key:     cfg.Secrets.RapidAPIKey,
		baseURL: "https://jsearch.p.rapidapi.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (p *Provider) Name() string { return "jsearch" }

// SearchJobs برای هر کلمه‌ی کلیدی جست‌وجو می‌کند و نتایج را ادغام می‌کند.
func (p *Provider) SearchJobs(ctx context.Context, f core.Filters) ([]core.Job, error) {
	queries := f.Keywords
	if len(queries) == 0 {
		queries = []string{"software developer"}
	}
	datePosted := datePostedFromHours(f.PostedWithinHours)

	var all []core.Job
	for _, q := range queries {
		jobs, err := p.searchOne(ctx, q, datePosted, f.RemoteOnly)
		if err != nil {
			return all, err
		}
		all = append(all, jobs...)
	}
	return all, nil
}

func (p *Provider) searchOne(ctx context.Context, query, datePosted string, remoteOnly bool) ([]core.Job, error) {
	u, _ := url.Parse(p.baseURL + "/search")
	q := u.Query()
	q.Set("query", query)
	q.Set("date_posted", datePosted)
	q.Set("page", "1")
	q.Set("num_pages", "1")
	if remoteOnly {
		q.Set("remote_jobs_only", "true")
	}
	u.RawQuery = q.Encode()

	var body []byte
	err := retry.Do(ctx, 3, 500*time.Millisecond, isRetryable, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-RapidAPI-Key", p.key)
		req.Header.Set("X-RapidAPI-Host", "jsearch.p.rapidapi.com")
		resp, err := p.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return &httpError{status: resp.StatusCode, body: string(body)}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("jsearch %q: %w", query, err)
	}
	return parse(body)
}

type rawResp struct {
	Data []rawJob `json:"data"`
}
type rawJob struct {
	JobID          string `json:"job_id"`
	JobTitle       string `json:"job_title"`
	Employer       string `json:"employer_name"`
	City           string `json:"job_city"`
	Country        string `json:"job_country"`
	IsRemote       bool   `json:"job_is_remote"`
	Description    string `json:"job_description"`
	ApplyLink      string `json:"job_apply_link"`
	EmploymentType string `json:"job_employment_type"`
	PostedAt       string `json:"job_posted_at_datetime_utc"`
}

func parse(body []byte) ([]core.Job, error) {
	var r rawResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse jsearch: %w", err)
	}
	jobs := make([]core.Job, 0, len(r.Data))
	for _, d := range r.Data {
		loc := d.City
		if d.Country != "" {
			if loc != "" {
				loc += ", "
			}
			loc += d.Country
		}
		jobs = append(jobs, core.Job{
			ID:             d.JobID,
			Title:          d.JobTitle,
			Company:        d.Employer,
			Location:       loc,
			Remote:         d.IsRemote,
			EmploymentType: d.EmploymentType,
			Description:    d.Description,
			URL:            d.ApplyLink,
			PostedAt:       parseTime(d.PostedAt),
			Source:         "jsearch",
		})
	}
	return jobs, nil
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func datePostedFromHours(h int) string {
	switch {
	case h <= 0:
		return "all"
	case h <= 24:
		return "today"
	case h <= 72:
		return "3days"
	case h <= 168:
		return "week"
	default:
		return "month"
	}
}

type httpError struct {
	status int
	body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("http %d: %s", e.status, e.body) }

func isRetryable(err error) bool {
	if he, ok := err.(*httpError); ok {
		return he.status == 429 || he.status >= 500
	}
	return true // خطاهای شبکه/timeout قابل‌تلاش مجددند
}
```

- [ ] **Step 3: تست و Commit**

```bash
go test ./internal/providers/jsearch/
git add internal/providers/jsearch && git commit -m "feat: add JSearch provider"
```
Expected: PASS.

---

## Task 9: Provider — LinkedIn

توجه: شکل پاسخ APIهای غیررسمی لینکدین روی RapidAPI متفاوت است. این پیاده‌سازی یک شمای
رایج را هدف می‌گیرد و host/path/key از config می‌آیند؛ در صورت اشتراک به API خاص، فقط
struct پاسخ (`rawJob`) کمی تنظیم می‌شود.

**Files:**
- Create: `internal/providers/linkedin/linkedin.go`, `internal/providers/linkedin/linkedin_test.go`

- [ ] **Step 1: تست**

Create `internal/providers/linkedin/linkedin_test.go`:

```go
package linkedin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

func TestSearchMapsJobs(t *testing.T) {
	var gotHost, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Header.Get("X-RapidAPI-Host")
		gotKey = r.Header.Get("X-RapidAPI-Key")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{
			"id":"li1","title":"Senior Go Engineer","company_name":"Stripe",
			"location":"Remote","url":"https://linkedin.com/jobs/li1","is_remote":true,
			"employment_type":"Full-time","seniority_level":"Senior",
			"description":"great role","posted_at":"2026-07-24T09:00:00Z"}]}`))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	p := &Provider{key: "k", host: host, path: "/search", scheme: "http", client: srv.Client()}
	jobs, err := p.SearchJobs(context.Background(), core.Filters{Keywords: []string{"go"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1, got %d", len(jobs))
	}
	j := jobs[0]
	if j.Title != "Senior Go Engineer" || j.Company != "Stripe" || j.Source != "linkedin" {
		t.Errorf("bad mapping: %+v", j)
	}
	if gotKey != "k" || gotHost != host {
		t.Errorf("headers: host=%q key=%q", gotHost, gotKey)
	}
}
```

- [ ] **Step 2: پیاده‌سازی**

Create `internal/providers/linkedin/linkedin.go`:

```go
// Package linkedin منبع لینکدین (API غیررسمی روی RapidAPI) را پیاده می‌کند.
// host/path/key از config می‌آیند تا با هر API لینکدینی که مشترک شدی کار کند.
package linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/retry"
)

func init() {
	providers.Register("linkedin", New)
}

// Provider منبع لینکدین.
type Provider struct {
	key    string
	host   string
	path   string
	scheme string
	client *http.Client
}

// New یک Provider از روی config می‌سازد.
func New(cfg core.Config) (providers.Provider, error) {
	s := cfg.Secrets
	if s.LinkedInKey == "" || s.LinkedInHost == "" {
		return nil, fmt.Errorf("LINKEDIN_RAPIDAPI_KEY and LINKEDIN_RAPIDAPI_HOST are required for linkedin")
	}
	path := s.LinkedInPath
	if path == "" {
		path = "/search"
	}
	return &Provider{
		key:    s.LinkedInKey,
		host:   s.LinkedInHost,
		path:   path,
		scheme: "https",
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (p *Provider) Name() string { return "linkedin" }

// SearchJobs برای هر کلمه‌ی کلیدی جست‌وجو می‌کند.
func (p *Provider) SearchJobs(ctx context.Context, f core.Filters) ([]core.Job, error) {
	queries := f.Keywords
	if len(queries) == 0 {
		queries = []string{"software engineer"}
	}
	location := ""
	if len(f.Countries) > 0 {
		location = f.Countries[0]
	}

	var all []core.Job
	for _, q := range queries {
		jobs, err := p.searchOne(ctx, q, location)
		if err != nil {
			return all, err
		}
		all = append(all, jobs...)
	}
	return all, nil
}

func (p *Provider) searchOne(ctx context.Context, keywords, location string) ([]core.Job, error) {
	u := &url.URL{Scheme: p.scheme, Host: p.host, Path: p.path}
	q := u.Query()
	q.Set("keywords", keywords)
	if location != "" {
		q.Set("location", location)
	}
	u.RawQuery = q.Encode()

	var body []byte
	err := retry.Do(ctx, 3, 500*time.Millisecond, isRetryable, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-RapidAPI-Key", p.key)
		req.Header.Set("X-RapidAPI-Host", p.host)
		resp, err := p.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return &httpError{status: resp.StatusCode, body: string(body)}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("linkedin %q: %w", keywords, err)
	}
	return parse(body)
}

// rawResp/rawJob — شمای فرض‌شده؛ در صورت نیاز با API خودت تنظیم کن.
type rawResp struct {
	Data []rawJob `json:"data"`
}
type rawJob struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Company        string `json:"company_name"`
	Location       string `json:"location"`
	URL            string `json:"url"`
	IsRemote       bool   `json:"is_remote"`
	EmploymentType string `json:"employment_type"`
	Seniority      string `json:"seniority_level"`
	Description    string `json:"description"`
	PostedAt       string `json:"posted_at"`
}

func parse(body []byte) ([]core.Job, error) {
	var r rawResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse linkedin: %w", err)
	}
	jobs := make([]core.Job, 0, len(r.Data))
	for _, d := range r.Data {
		jobs = append(jobs, core.Job{
			ID:             d.ID,
			Title:          d.Title,
			Company:        d.Company,
			Location:       d.Location,
			Remote:         d.IsRemote,
			EmploymentType: d.EmploymentType,
			Seniority:      d.Seniority,
			Description:    d.Description,
			URL:            d.URL,
			PostedAt:       parseTime(d.PostedAt),
			Source:         "linkedin",
		})
	}
	return jobs, nil
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

type httpError struct {
	status int
	body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("http %d: %s", e.status, e.body) }

func isRetryable(err error) bool {
	if he, ok := err.(*httpError); ok {
		return he.status == 429 || he.status >= 500
	}
	return true
}
```

- [ ] **Step 3: تست و Commit**

```bash
go test ./internal/providers/linkedin/
git add internal/providers/linkedin && git commit -m "feat: add configurable LinkedIn provider"
```
Expected: PASS.

---

## Task 10: `publisher` + Telegram

**Files:**
- Create: `internal/publisher/publisher.go`
- Create: `internal/publisher/telegram/telegram.go`, `internal/publisher/telegram/message.go`
- Test: `internal/publisher/telegram/telegram_test.go`, `internal/publisher/telegram/message_test.go`

- [ ] **Step 1: تست‌ها**

Create `internal/publisher/telegram/message_test.go`:

```go
package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/aghaie/job-finder/internal/core"
)

func TestFormatMessage(t *testing.T) {
	j := core.Job{
		Title:      "Senior Backend Engineer",
		Company:    "Stripe",
		Location:   "Remote (Worldwide)",
		Remote:     true,
		Relocation: true,
		URL:        "https://linkedin.com/jobs/1",
		PostedAt:   time.Now().Add(-3 * time.Hour),
	}
	out := formatMessage(j)
	for _, want := range []string{"Senior Backend Engineer", "Stripe", "Remote", "Relocation: Yes", "https://linkedin.com/jobs/1", "Posted:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
```

Create `internal/publisher/telegram/telegram_test.go`:

```go
package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

func TestPublishSends(t *testing.T) {
	var gotPath, gotChat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		r.ParseForm()
		gotChat = r.FormValue("chat_id")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := &Client{token: "123:ABC", chatID: "@ch", baseURL: srv.URL, client: srv.Client()}
	if err := c.Publish(context.Background(), core.Job{Title: "Dev", URL: "https://x.test"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.Contains(gotPath, "/bot123:ABC/sendMessage") {
		t.Errorf("path: %q", gotPath)
	}
	if gotChat != "@ch" {
		t.Errorf("chat: %q", gotChat)
	}
}

func TestPublishErrorOnNotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"description":"bad"}`))
	}))
	defer srv.Close()
	c := &Client{token: "t", chatID: "x", baseURL: srv.URL, client: srv.Client()}
	if err := c.Publish(context.Background(), core.Job{Title: "Dev"}); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: interface**

Create `internal/publisher/publisher.go`:

```go
// Package publisher رابط مقصد انتشار جاب‌ها.
package publisher

import (
	"context"

	"github.com/aghaie/job-finder/internal/core"
)

// Publisher یک مقصد انتشار (تلگرام و ...).
type Publisher interface {
	Publish(ctx context.Context, job core.Job) error
}
```

- [ ] **Step 3: قالب پیام**

Create `internal/publisher/telegram/message.go`:

```go
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
```

- [ ] **Step 4: کلاینت تلگرام**

Create `internal/publisher/telegram/telegram.go`:

```go
// Package telegram پیاده‌سازی Publisher برای کانال تلگرام.
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/publisher"
)

// Client فرستنده‌ی تلگرام.
type Client struct {
	token   string
	chatID  string
	baseURL string
	client  *http.Client
}

// New یک Client از روی config می‌سازد.
func New(cfg core.Config) (*Client, error) {
	s := cfg.Secrets
	if s.TelegramToken == "" || s.TelegramChatID == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required")
	}
	return &Client{
		token:   s.TelegramToken,
		chatID:  s.TelegramChatID,
		baseURL: "https://api.telegram.org",
		client:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// اطمینان از پیاده‌سازی interface.
var _ publisher.Publisher = (*Client)(nil)

type apiResp struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

// Publish یک جاب را به کانال می‌فرستد.
func (c *Client) Publish(ctx context.Context, job core.Job) error {
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.token)
	form := url.Values{}
	form.Set("chat_id", c.chatID)
	form.Set("text", formatMessage(job))
	form.Set("disable_web_page_preview", "false")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var ar apiResp
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

- [ ] **Step 5: تست و Commit**

```bash
go test ./internal/publisher/...
git add internal/publisher && git commit -m "feat: add publisher interface and Telegram implementation"
```
Expected: PASS.

---

## Task 11: `config` (env + JSON)

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

- [ ] **Step 1: تست**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{
		"providers":["jsearch"],
		"filters":{"keywords":["go"],"remoteOnly":true,"postedWithinHours":48},
		"maxPerRun":5,"delaySeconds":2,"allowInternship":false,"minDescriptionRunes":100
	}`), 0o644)

	t.Setenv("RAPIDAPI_KEY", "rk")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0] != "jsearch" {
		t.Errorf("providers: %+v", cfg.Providers)
	}
	if !cfg.Filters.RemoteOnly || cfg.Filters.PostedWithinHours != 48 {
		t.Errorf("filters: %+v", cfg.Filters)
	}
	if cfg.MaxPerRun != 5 || cfg.Secrets.RapidAPIKey != "rk" {
		t.Errorf("cfg: %+v", cfg)
	}
}

func TestLoadFailsWhenProviderSecretMissing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"providers":["jsearch"],"filters":{}}`), 0o644)
	t.Setenv("RAPIDAPI_KEY", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected error for missing RAPIDAPI_KEY")
	}
}
```

- [ ] **Step 2: پیاده‌سازی**

Create `internal/config/config.go`:

```go
// Package config تنظیمات را از فایل JSON (فیلترها) و env (رازها) می‌خواند.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aghaie/job-finder/internal/core"
)

type fileConfig struct {
	Providers           []string     `json:"providers"`
	Filters             core.Filters `json:"filters"`
	MaxPerRun           int          `json:"maxPerRun"`
	DelaySeconds        int          `json:"delaySeconds"`
	AllowInternship     bool         `json:"allowInternship"`
	MinDescriptionRunes int          `json:"minDescriptionRunes"`
}

// Load فایل config و رازهای env را می‌خواند و اعتبارسنجی می‌کند.
func Load(path string) (core.Config, error) {
	if path == "" {
		path = "config.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return core.Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return core.Config{}, fmt.Errorf("parse config: %w", err)
	}

	cfg := core.Config{
		Providers:           fc.Providers,
		Filters:             fc.Filters,
		MaxPerRun:           orDefault(fc.MaxPerRun, 20),
		DelaySeconds:        orDefault(fc.DelaySeconds, 4),
		AllowInternship:     fc.AllowInternship,
		MinDescriptionRunes: orDefault(fc.MinDescriptionRunes, 200),
		SeenPath:            getEnv("SEEN_PATH", "data/seen_jobs.json"),
		MaxSeen:            orDefault(atoiEnv("MAX_SEEN"), 5000),
		Secrets: core.Secrets{
			RapidAPIKey:    os.Getenv("RAPIDAPI_KEY"),
			LinkedInKey:    os.Getenv("LINKEDIN_RAPIDAPI_KEY"),
			LinkedInHost:   os.Getenv("LINKEDIN_RAPIDAPI_HOST"),
			LinkedInPath:   getEnv("LINKEDIN_RAPIDAPI_PATH", "/search"),
			TelegramToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
			TelegramChatID: os.Getenv("TELEGRAM_CHAT_ID"),
		},
	}
	if len(cfg.Providers) == 0 {
		return cfg, fmt.Errorf("config: providers list is empty")
	}
	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validate(cfg core.Config) error {
	var missing []string
	if cfg.Secrets.TelegramToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if cfg.Secrets.TelegramChatID == "" {
		missing = append(missing, "TELEGRAM_CHAT_ID")
	}
	for _, p := range cfg.Providers {
		switch p {
		case "jsearch":
			if cfg.Secrets.RapidAPIKey == "" {
				missing = append(missing, "RAPIDAPI_KEY (for jsearch)")
			}
		case "linkedin":
			if cfg.Secrets.LinkedInKey == "" {
				missing = append(missing, "LINKEDIN_RAPIDAPI_KEY (for linkedin)")
			}
			if cfg.Secrets.LinkedInHost == "" {
				missing = append(missing, "LINKEDIN_RAPIDAPI_HOST (for linkedin)")
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func atoiEnv(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
```

- [ ] **Step 3: تست و Commit**

```bash
go test ./internal/config/
git add internal/config && git commit -m "feat: add config loader (JSON file + env secrets)"
```
Expected: PASS.

---

## Task 12: `pipeline`

**Files:**
- Create: `internal/pipeline/pipeline.go`, `internal/pipeline/pipeline_test.go`

- [ ] **Step 1: تست**

Create `internal/pipeline/pipeline_test.go`:

```go
package pipeline

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/filter"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/store"
)

type fakeProvider struct {
	name string
	jobs []core.Job
	err  error
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) SearchJobs(context.Context, core.Filters) ([]core.Job, error) {
	return f.jobs, f.err
}

type fakePublisher struct{ sent []core.Job }

func (f *fakePublisher) Publish(_ context.Context, j core.Job) error {
	f.sent = append(f.sent, j)
	return nil
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func allowAllChain() *filter.Chain { return filter.NewChain(quiet()) }

func TestRunDedupsFiltersAndPublishes(t *testing.T) {
	jobs := []core.Job{
		{ID: "1", Title: "Backend Engineer", Company: "A", URL: "https://a.test/1", Remote: true, Description: "x"},
		{ID: "1dup", Title: "Backend Engineer", Company: "A", URL: "https://www.a.test/1/", Remote: true, Description: "x"},
		{ID: "2", Title: "Frontend Dev", Company: "B", URL: "https://b.test/2", Remote: true, Description: "y"},
	}
	prov := fakeProvider{name: "fake", jobs: jobs}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	p := New([]providers.Provider{prov}, allowAllChain(), st, pub, quiet(), core.Config{MaxPerRun: 10, DelaySeconds: 0})
	stats, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(pub.sent) != 2 {
		t.Fatalf("expected 2 published (after dedup), got %d", len(pub.sent))
	}
	if stats.Published != 2 {
		t.Errorf("stats.Published = %d", stats.Published)
	}

	// اجرای دوم: همه دیده‌شده‌اند
	pub2 := &fakePublisher{}
	p2 := New([]providers.Provider{prov}, allowAllChain(), st, pub2, quiet(), core.Config{MaxPerRun: 10, DelaySeconds: 0})
	if _, err := p2.Run(context.Background()); err != nil {
		t.Fatalf("run2: %v", err)
	}
	if len(pub2.sent) != 0 {
		t.Errorf("expected 0 on second run, got %d", len(pub2.sent))
	}
}

func TestRunIsolatesProviderError(t *testing.T) {
	good := fakeProvider{name: "good", jobs: []core.Job{{ID: "1", Title: "Dev", Company: "A", URL: "https://a.test/1", Remote: true, Description: "x"}}}
	bad := fakeProvider{name: "bad", err: context.DeadlineExceeded}
	pub := &fakePublisher{}
	st, _ := store.NewFileStore(t.TempDir()+"/seen.json", 100)

	p := New([]providers.Provider{good, bad}, allowAllChain(), st, pub, quiet(), core.Config{MaxPerRun: 10})
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatalf("run should not fail on one provider error: %v", err)
	}
	if len(pub.sent) != 1 {
		t.Errorf("expected 1 published from good provider, got %d", len(pub.sent))
	}
}
```

- [ ] **Step 2: پیاده‌سازی**

Create `internal/pipeline/pipeline.go`:

```go
// Package pipeline مراحل گرفتن، نرمال‌سازی، dedup، فیلتر و انتشار را هماهنگ می‌کند.
package pipeline

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/dedup"
	"github.com/aghaie/job-finder/internal/filter"
	"github.com/aghaie/job-finder/internal/normalize"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/publisher"
	"github.com/aghaie/job-finder/internal/store"
)

// Stats خلاصه‌ی یک اجرا.
type Stats struct {
	Fetched   int
	AfterDedup int
	AfterSeen int
	Published int
}

// Pipeline اجزای سرویس را نگه می‌دارد.
type Pipeline struct {
	providers []providers.Provider
	chain     *filter.Chain
	store     store.SeenStore
	pub       publisher.Publisher
	log       *slog.Logger
	filters   core.Filters
	maxPerRun int
	delay     time.Duration
}

// New یک Pipeline می‌سازد.
func New(provs []providers.Provider, chain *filter.Chain, st store.SeenStore, pub publisher.Publisher, log *slog.Logger, cfg core.Config) *Pipeline {
	return &Pipeline{
		providers: provs,
		chain:     chain,
		store:     st,
		pub:       pub,
		log:       log,
		filters:   cfg.Filters,
		maxPerRun: cfg.MaxPerRun,
		delay:     time.Duration(cfg.DelaySeconds) * time.Second,
	}
}

// Run یک چرخه‌ی کامل را اجرا می‌کند.
func (p *Pipeline) Run(ctx context.Context) (Stats, error) {
	var stats Stats

	// ۱) گرفتن موازی از همه Providerها (Error Isolation)
	all := p.fetchAll(ctx)
	stats.Fetched = len(all)

	// ۲) نرمال‌سازی
	for i := range all {
		all[i] = normalize.Job(all[i])
	}

	// ۳+۴) dedup داخل‌اجرا
	deduped := dedup.Dedupe(all)
	stats.AfterDedup = len(deduped)

	// ۵) حذف دیده‌شده‌ها
	var fresh []core.Job
	for _, j := range deduped {
		if !p.store.IsSeen(j.Fingerprint()) {
			fresh = append(fresh, j)
		}
	}
	stats.AfterSeen = len(fresh)

	// ۶+۷) فیلتر و انتشار
	for _, j := range fresh {
		if p.maxPerRun > 0 && stats.Published >= p.maxPerRun {
			p.log.Info("reached maxPerRun", "max", p.maxPerRun)
			break
		}
		if !p.chain.Allow(ctx, j) {
			continue
		}
		if err := p.pub.Publish(ctx, j); err != nil {
			p.log.Warn("publish failed", "job", j.Title, "err", err)
			continue // دیده‌شده علامت نمی‌زنیم تا دفعه‌ی بعد دوباره تلاش شود
		}
		p.store.MarkSeen(j.Fingerprint())
		stats.Published++
		if p.delay > 0 {
			select {
			case <-ctx.Done():
				break
			case <-time.After(p.delay):
			}
		}
	}

	// ۸) ذخیره
	if err := p.store.Save(ctx); err != nil {
		return stats, err
	}
	p.log.Info("run complete",
		"fetched", stats.Fetched, "afterDedup", stats.AfterDedup,
		"afterSeen", stats.AfterSeen, "published", stats.Published)
	return stats, nil
}

func (p *Pipeline) fetchAll(ctx context.Context) []core.Job {
	var (
		mu  sync.Mutex
		all []core.Job
		wg  sync.WaitGroup
	)
	for _, prov := range p.providers {
		wg.Add(1)
		go func(pr providers.Provider) {
			defer wg.Done()
			jobs, err := pr.SearchJobs(ctx, p.filters)
			if err != nil {
				p.log.Warn("provider fetch failed", "provider", pr.Name(), "err", err)
			}
			if len(jobs) > 0 {
				mu.Lock()
				all = append(all, jobs...)
				mu.Unlock()
			}
			p.log.Info("provider fetched", "provider", pr.Name(), "count", len(jobs))
		}(prov)
	}
	wg.Wait()
	return all
}
```

- [ ] **Step 3: تست و Commit**

```bash
go test ./internal/pipeline/
git add internal/pipeline && git commit -m "feat: add pipeline orchestrator"
```
Expected: PASS.

---

## Task 13: `main` (Composition Root) + بیلد کامل

**Files:**
- Create: `cmd/jobfinder/main.go`

- [ ] **Step 1: نوشتن main**

Create `cmd/jobfinder/main.go`:

```go
// Command jobfinder سرویس را اجرا می‌کند: گرفتن، فیلتر و انتشار جاب‌های جدید.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aghaie/job-finder/internal/config"
	"github.com/aghaie/job-finder/internal/filter"
	"github.com/aghaie/job-finder/internal/pipeline"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/publisher/telegram"
	"github.com/aghaie/job-finder/internal/store"

	// ثبت Providerها (افزودن منبع جدید = یک خط blank-import اینجا).
	_ "github.com/aghaie/job-finder/internal/providers/jsearch"
	_ "github.com/aghaie/job-finder/internal/providers/linkedin"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load(os.Getenv("CONFIG_PATH"))
	if err != nil {
		log.Error("config error", "err", err)
		os.Exit(1)
	}

	provs, err := providers.Build(cfg.Providers, cfg)
	if err != nil {
		log.Error("provider build error", "err", err)
		os.Exit(1)
	}

	st, err := store.NewFileStore(cfg.SeenPath, cfg.MaxSeen)
	if err != nil {
		log.Error("store error", "err", err)
		os.Exit(1)
	}

	pub, err := telegram.New(cfg)
	if err != nil {
		log.Error("publisher error", "err", err)
		os.Exit(1)
	}

	chain := filter.BuildRuleChain(log, cfg)
	p := pipeline.New(provs, chain, st, pub, log, cfg)

	if _, err := p.Run(context.Background()); err != nil {
		log.Error("run error", "err", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: بیلد/vet/تست کامل**

```bash
export PATH="$HOME/sdk/go/bin:$PATH"; export GOTOOLCHAIN=local
gofmt -l .
go build ./...
go vet ./...
go test ./...
```
Expected: gofmt هیچ فایلی چاپ نکند، build/vet بدون خطا، همه تست‌ها PASS.

- [ ] **Step 3: تست دود (خطای پیکربندی تمیز)**

```bash
env -u RAPIDAPI_KEY -u TELEGRAM_BOT_TOKEN -u TELEGRAM_CHAT_ID CONFIG_PATH=/nonexistent go run ./cmd/jobfinder; echo "exit: $?"
```
Expected: پیام خطای config و exit غیرصفر.

- [ ] **Step 4: Commit**

```bash
git add cmd && git commit -m "feat: add composition root wiring all components"
```

---

## Task 14: فایل‌های تنظیمات، وضعیت، Workflow و README

**Files:**
- Create: `config.example.json`, `data/seen_jobs.json`
- Modify: `.env.example`, `.github/workflows/daily.yml`, `README.md`

- [ ] **Step 1: نمونه config**

Create `config.example.json`:

```json
{
  "providers": ["linkedin", "jsearch"],
  "filters": {
    "countries": ["US", "DE", "NL"],
    "keywords": ["developer", "engineer", "backend", "golang"],
    "excludeKeywords": ["sales", "manager"],
    "employmentTypes": ["FULLTIME", "CONTRACTOR"],
    "seniority": ["mid", "senior"],
    "remoteOnly": false,
    "relocationOnly": false,
    "postedWithinHours": 48,
    "companyWhitelist": [],
    "companyBlacklist": []
  },
  "maxPerRun": 20,
  "delaySeconds": 4,
  "allowInternship": false,
  "minDescriptionRunes": 200
}
```

- [ ] **Step 2: فایل وضعیت اولیه**

Create `data/seen_jobs.json`:

```json
{
  "seen": []
}
```

- [ ] **Step 3: به‌روزرسانی `.env.example`**

Replace `.env.example` content with:

```
# JSearch (RapidAPI)
RAPIDAPI_KEY=your_rapidapi_key

# LinkedIn provider (RapidAPI) — host همان API لینکدینی که مشترک شدی
LINKEDIN_RAPIDAPI_KEY=your_linkedin_rapidapi_key
LINKEDIN_RAPIDAPI_HOST=some-linkedin-api.p.rapidapi.com
# LINKEDIN_RAPIDAPI_PATH=/search

# Telegram
TELEGRAM_BOT_TOKEN=123456:ABC-your-bot-token
TELEGRAM_CHAT_ID=@yourchannel

# اختیاری
# CONFIG_PATH=config.json
# SEEN_PATH=data/seen_jobs.json
# MAX_SEEN=5000
```

- [ ] **Step 4: به‌روزرسانی workflow**

Replace `.github/workflows/daily.yml` content with:

```yaml
name: daily-job-finder

on:
  schedule:
    - cron: "0 6 * * *"   # روزی یک بار، ۰۶:۰۰ UTC
  workflow_dispatch: {}

permissions:
  contents: write

concurrency:
  group: job-finder
  cancel-in-progress: false

jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - name: Run
        env:
          RAPIDAPI_KEY: ${{ secrets.RAPIDAPI_KEY }}
          LINKEDIN_RAPIDAPI_KEY: ${{ secrets.LINKEDIN_RAPIDAPI_KEY }}
          LINKEDIN_RAPIDAPI_HOST: ${{ secrets.LINKEDIN_RAPIDAPI_HOST }}
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

- [ ] **Step 5: به‌روزرسانی README**

Replace `README.md` content with:

````markdown
# Job Finder → Telegram (Modular Pipeline)

سرویس ماژولار که جاب‌های Remote/Relocation باکیفیت را از چند منبع پیدا و در تلگرام منتشر می‌کند.

## معماری

```
Providers → Normalize → Merge → Dedup → Filter → Publish(Telegram)
```

هر منبع یک `Provider`، هر فیلتر یک `Filter`، ذخیره‌سازی یک `SeenStore`، انتشار یک `Publisher`.
افزودن منبع جدید = یک فولدر در `internal/providers/` + یک خط blank-import در `cmd/jobfinder/main.go`.

## پیش‌نیازها

- کلید RapidAPI برای JSearch و/یا یک LinkedIn Jobs API.
- بات تلگرام (BotFather) + کانال (بات ادمین باشد).

## اجرای محلی

```bash
cp .env.example .env          # رازها را پر کن
cp config.example.json config.json   # فیلترها را تنظیم کن
set -a && source .env && set +a
go run ./cmd/jobfinder
```

## تنظیمات

- **رازها** فقط در `.env` / GitHub Secrets: `RAPIDAPI_KEY`, `LINKEDIN_RAPIDAPI_KEY`,
  `LINKEDIN_RAPIDAPI_HOST`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`.
- **فیلترها و منابع** در `config.json` (countries، keywords، remoteOnly، ... ).

## اجرای خودکار

`.github/workflows/daily.yml` روزی یک بار اجرا می‌شود (قابل‌تنظیم) و وضعیت را commit می‌کند.
Secretها را در Settings → Secrets and variables → Actions ثبت کن.

## فازهای بعدی

- فاز ۲: فیلتر AI با Claude (پشت همان `Filter`).
- فاز ۳: Providerهای Greenhouse / Lever / Ashby / Workday.
- فاز ۴: ذخیره‌سازی Upstash Redis (پشت همان `SeenStore`).
````

- [ ] **Step 6: بیلد نهایی و Commit**

```bash
export PATH="$HOME/sdk/go/bin:$PATH"; export GOTOOLCHAIN=local
go build ./... && go test ./...
git add -A && git commit -m "chore: add config example, state, workflow, and README"
```
Expected: PASS.

---

## Self-Review Notes

- **پوشش اسپک:** همه‌ی اجزای سند معماری (core, config, providers+registry, jsearch, linkedin,
  normalize, dedup, filter chain, store, publisher/telegram, pipeline, retry, ratelimit, main,
  workflow) به تسک نگاشت شده‌اند.
- **سازگاری نوع‌ها:** `core.Job`/`core.Filters`/`core.Config`/`core.Decision` یک‌بار تعریف و
  همه‌جا استفاده می‌شوند. امضاها: `Provider.SearchJobs(ctx, core.Filters)`,
  `providers.Build(names, core.Config)`, `filter.BuildRuleChain(log, cfg)`,
  `Chain.Allow(ctx, job)`, `store.NewFileStore(path, maxSeen)`, `telegram.New(cfg)`,
  `pipeline.New(provs, chain, st, pub, log, cfg)`, `Pipeline.Run(ctx) (Stats, error)` — در main هماهنگ.
- **ماژولار بودن:** افزودن Provider جدید فقط فولدر + یک blank-import.
- **صفر وابستگی خارجی:** فقط stdlib؛ بیلد محلی روی شبکه‌ی محدود کار می‌کند.
- **ratelimit:** در فاز ۱ کمکی آماده است ولی برای اجرای روزانه در providerها استفاده‌ی سنگین
  ندارد؛ retry در هر دو provider استفاده شده است.
