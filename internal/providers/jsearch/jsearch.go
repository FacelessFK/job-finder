// Package jsearch منبع JSearch (RapidAPI) را پیاده می‌کند.
package jsearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/ratelimit"
	"github.com/aghaie/job-finder/internal/retry"
)

func init() {
	providers.Register("jsearch", New)
}

// Provider منبع JSearch.
type Provider struct {
	keys     []string
	baseURL  string
	client   *http.Client
	numPages int
	limiter  *ratelimit.Limiter

	mu      sync.Mutex
	keyIdx  int          // کلید فعالِ فعلی
	retired map[int]bool // کلیدهایی که سهمیه‌شان تمام شده یا نامعتبرند
}

// New یک Provider از روی config می‌سازد.
func New(cfg core.Config) (providers.Provider, error) {
	if len(cfg.Secrets.RapidAPIKeys) == 0 {
		return nil, fmt.Errorf("RAPIDAPI_KEY or RAPIDAPI_KEYS is required for jsearch")
	}
	pages := cfg.NumPages
	if pages <= 0 {
		pages = 1
	}
	return &Provider{
		keys:     cfg.Secrets.RapidAPIKeys,
		baseURL:  "https://jsearch.p.rapidapi.com",
		client:   &http.Client{Timeout: 30 * time.Second},
		numPages: pages,
		retired:  map[int]bool{},
		// حالا ضربِ کشور در کلمه‌ی کلیدی چند درخواست پشت‌سرهم می‌سازد؛
		// بدون فاصله‌گذاری، سقفِ نرخِ RapidAPI فعال می‌شود.
		limiter: ratelimit.New(300 * time.Millisecond),
	}, nil
}

// nextLiveKey کلید فعالِ بعدی را می‌دهد؛ اگر همه بازنشسته شده باشند false.
func (p *Provider) nextLiveKey() (int, string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < len(p.keys); i++ {
		idx := (p.keyIdx + i) % len(p.keys)
		if !p.retired[idx] {
			p.keyIdx = idx
			return idx, p.keys[idx], true
		}
	}
	return 0, "", false
}

// retire یک کلید را کنار می‌گذارد تا در ادامه‌ی همین اجرا دوباره امتحان نشود.
func (p *Provider) retire(idx int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.retired == nil {
		p.retired = map[int]bool{}
	}
	p.retired[idx] = true
	p.keyIdx = (idx + 1) % len(p.keys)
}

func (p *Provider) Name() string { return "jsearch" }

// SearchJobs ضرب کلمات کلیدی در کشورها را جست‌وجو می‌کند و نتایج را ادغام می‌کند.
// شکست یک جست‌وجو بقیه را متوقف نمی‌کند؛ خطاها جمع و در پایان برگردانده می‌شوند.
func (p *Provider) SearchJobs(ctx context.Context, f core.Filters) ([]core.Job, error) {
	queries := f.SearchQueries
	if len(queries) == 0 {
		queries = f.Keywords
	}
	if len(queries) == 0 {
		queries = []string{"software developer"}
	}
	// رشته‌ی خالی یعنی «بدون قید کشور» تا رفتار قبلی حفظ شود.
	countries := normalizeCountries(f.Countries)
	datePosted := datePostedFromHours(f.PostedWithinHours)

	var (
		all  []core.Job
		seen = map[string]bool{}
		errs []error
	)
	for _, c := range countries {
		for _, q := range queries {
			jobs, err := p.searchOne(ctx, q, c, datePosted, f.RemoteOnly)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for _, j := range jobs {
				fp := j.Fingerprint()
				if seen[fp] {
					continue
				}
				seen[fp] = true
				all = append(all, j)
			}
		}
	}
	return all, errors.Join(errs...)
}

// normalizeCountries کدها را کوچک و یکتا می‌کند؛ فهرست خالی یک جست‌وجوی بی‌قید می‌دهد.
func normalizeCountries(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, c := range in {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func (p *Provider) searchOne(ctx context.Context, query, country, datePosted string, remoteOnly bool) ([]core.Job, error) {
	u, _ := url.Parse(p.baseURL + "/search-v2")
	q := u.Query()
	q.Set("query", query)
	q.Set("date_posted", datePosted)
	q.Set("page", "1")
	q.Set("num_pages", strconv.Itoa(p.numPages))
	if country != "" {
		q.Set("country", country)
	}
	if remoteOnly {
		q.Set("remote_jobs_only", "true")
	}
	u.RawQuery = q.Encode()

	// هر کلید یک‌بار امتحان می‌شود؛ کلیدِ سوخته بازنشسته و بعدی جایگزین می‌شود.
	for range p.keys {
		idx, key, ok := p.nextLiveKey()
		if !ok {
			break
		}
		body, err := p.doRequest(ctx, u.String(), key)
		if err == nil {
			return parse(body, country)
		}
		if isKeyDead(err) {
			p.retire(idx)
			continue
		}
		return nil, fmt.Errorf("jsearch %q: %w", query, err)
	}
	return nil, fmt.Errorf("jsearch %q: all %d API keys exhausted", query, len(p.keys))
}

func (p *Provider) doRequest(ctx context.Context, url, key string) ([]byte, error) {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	var body []byte
	err := retry.Do(ctx, 3, 500*time.Millisecond, isRetryable, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-RapidAPI-Key", key)
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
	return body, err
}

// isKeyDead یعنی مشکل از خودِ کلید است و تلاش دوباره با همان کلید بی‌فایده:
// سهمیه‌ی ماهانه تمام شده یا اشتراک نامعتبر است.
func isKeyDead(err error) bool {
	var he *httpError
	if !errors.As(err, &he) {
		return false
	}
	if he.status == http.StatusForbidden || he.status == http.StatusUnauthorized {
		return true
	}
	// ۴۲۹ دو معنی دارد: سقف لحظه‌ای (قابل تلاش مجدد) یا سهمیه‌ی ماهانه (نه).
	return he.status == http.StatusTooManyRequests &&
		strings.Contains(strings.ToLower(he.body), "quota")
}

type rawResp struct {
	Data struct {
		Jobs []rawJob `json:"jobs"`
	} `json:"data"`
}
type rawJob struct {
	JobID       string `json:"job_id"`
	JobTitle    string `json:"job_title"`
	Employer    string `json:"employer_name"`
	City        string `json:"job_city"`
	Country     string `json:"job_country"`
	Location    string `json:"job_location"`
	IsRemote    bool   `json:"job_is_remote"`
	Description string `json:"job_description"`
	ApplyLink   string `json:"job_apply_link"`
	// EmploymentType در پاسخ‌های واقعی بومی‌شده است ("Vollzeit")؛
	// کد استاندارد فقط در آرایه می‌آید.
	EmploymentType  string   `json:"job_employment_type"`
	EmploymentTypes []string `json:"job_employment_types"`
	PostedAt        string   `json:"job_posted_at_datetime_utc"`
	PostedAtUnix    int64    `json:"job_posted_at_timestamp"`
}

// employmentType کد استاندارد را ترجیح می‌دهد و فقط در نبودش به رشته‌ی
// بومی‌شده برمی‌گردد.
func (d rawJob) employmentType() string {
	if len(d.EmploymentTypes) > 0 && d.EmploymentTypes[0] != "" {
		return d.EmploymentTypes[0]
	}
	return d.EmploymentType
}

// location وقتی شهر و کشور خالی‌اند از job_location استفاده می‌کند.
func (d rawJob) location() string {
	loc := d.City
	if d.Country != "" {
		if loc != "" {
			loc += ", "
		}
		loc += d.Country
	}
	if loc == "" {
		return d.Location
	}
	return loc
}

func (d rawJob) postedAt() time.Time {
	if t := parseTime(d.PostedAt); !t.IsZero() {
		return t
	}
	if d.PostedAtUnix > 0 {
		return time.Unix(d.PostedAtUnix, 0).UTC()
	}
	return time.Time{}
}

// parse پاسخ را به مدل دامنه تبدیل می‌کند. searchedCountry کشوری است که
// درخواست کردیم؛ در پاسخ‌های واقعی job_country اغلب خالی است و بدون این
// جایگزین، فیلتر کشور عملاً کور می‌شود.
func parse(body []byte, searchedCountry string) ([]core.Job, error) {
	var r rawResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse jsearch: %w", err)
	}
	jobs := make([]core.Job, 0, len(r.Data.Jobs))
	for _, d := range r.Data.Jobs {
		country := d.Country
		if country == "" {
			country = strings.ToUpper(searchedCountry)
		}
		jobs = append(jobs, core.Job{
			ID:             d.JobID,
			Title:          d.JobTitle,
			Company:        d.Employer,
			Location:       d.location(),
			Country:        country,
			Remote:         d.IsRemote,
			EmploymentType: d.employmentType(),
			Description:    d.Description,
			URL:            d.ApplyLink,
			PostedAt:       d.postedAt(),
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
		if isKeyDead(he) {
			return false // تلاش دوباره با کلیدِ سوخته فقط وقت تلف می‌کند
		}
		return he.status == 429 || he.status >= 500
	}
	return true // خطاهای شبکه/timeout قابل‌تلاش مجددند
}
