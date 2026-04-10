package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func TestGetCustomer_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/customers/ABC123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"customer": map[string]any{
				"id":            "ABC123",
				"given_name":    "Jane",
				"family_name":   "Doe",
				"email_address": "jane@example.com",
			},
		})
	}))
	defer srv.Close()

	c := &Client{SDK: squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL))}

	got, err := c.GetCustomer(context.Background(), "ABC123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == nil || *got.ID != "ABC123" {
		t.Errorf("got ID=%v, want ABC123", got.ID)
	}
	if got.EmailAddress == nil || *got.EmailAddress != "jane@example.com" {
		t.Errorf("got Email=%v, want jane@example.com", got.EmailAddress)
	}
}

func TestGetCustomer_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{SDK: squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL))}

	_, err := c.GetCustomer(context.Background(), "NOTEXIST")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestGetCustomer_EmptyID(t *testing.T) {
	c := &Client{SDK: squareclient.NewClient(option.WithToken("tok"))}

	_, err := c.GetCustomer(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}
