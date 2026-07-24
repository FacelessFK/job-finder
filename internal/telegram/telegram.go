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
