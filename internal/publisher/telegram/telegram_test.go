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
