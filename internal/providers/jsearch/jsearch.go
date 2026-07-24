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
	u, _ := url.Parse(p.baseURL + "/search-v2")
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
	Data struct {
		Jobs []rawJob `json:"jobs"`
	} `json:"data"`
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
	jobs := make([]core.Job, 0, len(r.Data.Jobs))
	for _, d := range r.Data.Jobs {
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
