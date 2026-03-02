package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCustomer_Success(t *testing.T) {
	want := Customer{
		ID:           "ABC123",
		GivenName:    "Jane",
		FamilyName:   "Doe",
		EmailAddress: "jane@example.com",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/customers/ABC123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(getCustomerResponse{Customer: want})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, accessToken: "tok", http: &http.Client{}}

	got, err := c.GetCustomer(context.Background(), "ABC123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID || got.EmailAddress != want.EmailAddress {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestGetCustomer_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, accessToken: "tok", http: &http.Client{}}

	_, err := c.GetCustomer(context.Background(), "NOTEXIST")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestGetCustomer_EmptyID(t *testing.T) {
	c := &Client{baseURL: "http://unused", accessToken: "tok", http: &http.Client{}}

	_, err := c.GetCustomer(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestGetCustomer_PathEscapesID(t *testing.T) {
	captured := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// RawPath holds the encoded form; Path is decoded by the HTTP server.
		captured = r.URL.RawPath
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(getCustomerResponse{})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, accessToken: "tok", http: &http.Client{}}
	c.GetCustomer(context.Background(), "id/with/slashes")

	if captured != "/v2/customers/id%2Fwith%2Fslashes" {
		t.Errorf("path not escaped correctly: %s", captured)
	}
}
