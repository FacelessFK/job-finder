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
