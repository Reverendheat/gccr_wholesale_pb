package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestGetCustomerWholesaleAudiences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"customer": map[string]any{
			"id":        "CUSTOMER",
			"group_ids": []string{"UNRELATED", "CAFE_GROUP", "GROCERY_GROUP"},
		}})
	}))
	defer srv.Close()

	client := &Client{
		SDK:                            squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL)),
		WholesaleGroceryGroupID:        "GROCERY_GROUP",
		WholesaleCafeRestaurantGroupID: "CAFE_GROUP",
	}
	got, err := client.GetCustomerWholesaleAudiences(context.Background(), "CUSTOMER")
	if err != nil {
		t.Fatal(err)
	}
	want := []WholesaleAudience{WholesaleAudienceGrocery, WholesaleAudienceCafeRestaurant}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("audiences = %#v, want %#v", got, want)
	}
}

func TestSetCustomerWholesaleAudiencesUpdatesConfiguredGroupsOnly(t *testing.T) {
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/customers/CUSTOMER":
			_ = json.NewEncoder(w).Encode(map[string]any{"customer": map[string]any{
				"id":        "CUSTOMER",
				"group_ids": []string{"UNRELATED", "GROCERY_GROUP"},
			}})
		case "/v2/customers/CUSTOMER/groups/GROCERY_GROUP", "/v2/customers/CUSTOMER/groups/CAFE_GROUP":
			_ = json.NewEncoder(w).Encode(map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &Client{
		SDK:                            squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL)),
		WholesaleGroceryGroupID:        "GROCERY_GROUP",
		WholesaleCafeRestaurantGroupID: "CAFE_GROUP",
	}
	got, err := client.SetCustomerWholesaleAudiences(context.Background(), "CUSTOMER", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []WholesaleAudience{WholesaleAudienceCafeRestaurant}; !reflect.DeepEqual(got, want) {
		t.Fatalf("audiences = %#v, want %#v", got, want)
	}
	wantRequests := []string{
		"GET /v2/customers/CUSTOMER",
		"DELETE /v2/customers/CUSTOMER/groups/GROCERY_GROUP",
		"PUT /v2/customers/CUSTOMER/groups/CAFE_GROUP",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
}
