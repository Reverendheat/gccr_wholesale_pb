package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew_SandboxURL(t *testing.T) {
	c := New(Config{AccessToken: "tok", Sandbox: true})
	if c.baseURL != sandboxBaseURL {
		t.Errorf("expected sandbox URL %s, got %s", sandboxBaseURL, c.baseURL)
	}
}

func TestNew_ProductionURL(t *testing.T) {
	c := New(Config{AccessToken: "tok", Sandbox: false})
	if c.baseURL != productionBaseURL {
		t.Errorf("expected production URL %s, got %s", productionBaseURL, c.baseURL)
	}
}

func TestDoGet_Success(t *testing.T) {
	want := map[string]string{"id": "abc123"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or wrong Authorization header: %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, accessToken: "test-token", http: &http.Client{}}

	var got map[string]string
	if err := c.doGet(context.Background(), "/v2/catalog/list", &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["id"] != want["id"] {
		t.Errorf("expected id %q, got %q", want["id"], got["id"])
	}
}

func TestDoGet_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, accessToken: "bad-token", http: &http.Client{}}

	var got any
	err := c.doGet(context.Background(), "/v2/catalog/list", &got)
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}
