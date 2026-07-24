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
