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
