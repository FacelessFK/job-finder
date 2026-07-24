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
	Status string   `json:"status"`
	Data   []rawJob `json:"data"`
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
