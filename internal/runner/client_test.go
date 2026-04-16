package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestNewClient_UsesProxyURL(t *testing.T) {
	var proxied atomic.Bool
	var targetHit atomic.Bool

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit.Store(true)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer target.Close()

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Store(true)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()

	client, err := NewClient(&Config{ProxyURL: proxy.URL})
	if err != nil {
		t.Fatalf("NewClient error = %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, target.URL+"/widgets", nil)
	if err != nil {
		t.Fatalf("http.NewRequest error = %v", err)
	}
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("client.Do error = %v", err)
	}
	defer resp.Body.Close()

	if !proxied.Load() {
		t.Fatal("expected request to go through proxy")
	}
	if targetHit.Load() {
		t.Fatal("expected proxy to intercept request before target")
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestNewClient_InvalidProxyURL(t *testing.T) {
	if _, err := NewClient(&Config{ProxyURL: "://bad proxy"}); err == nil {
		t.Fatal("expected proxy URL parse error")
	}
}
