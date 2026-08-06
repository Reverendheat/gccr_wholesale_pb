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

func TestGetWholesaleCatalog_UsesConfiguredCategory(t *testing.T) {
	const categoryID = "configured-category-id"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/catalog/search-catalog-items" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify the request body contains the wholesale category ID
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("could not decode request body: %v", err)
		}
		catIDs, ok := body["category_ids"].([]any)
		if !ok || len(catIDs) == 0 || catIDs[0] != categoryID {
			t.Errorf("expected configured category ID in request, got %v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"id": "ITEM1", "type": "ITEM",
					"item_data": map[string]any{
						"name": "Item 1",
						"variations": []map[string]any{
							{"id": "VAR1", "type": "ITEM_VARIATION", "item_variation_data": map[string]any{
								"item_id": "ITEM1", "name": "Default", "pricing_type": "FIXED_PRICING",
								"price_money": map[string]any{"amount": 500, "currency": "USD"},
							}},
						},
					},
				},
				{
					"id": "ITEM2", "type": "ITEM",
					"item_data": map[string]any{
						"name": "Item 2",
						"variations": []map[string]any{
							{"id": "VAR2", "type": "ITEM_VARIATION", "item_variation_data": map[string]any{
								"item_id": "ITEM2", "name": "Default", "pricing_type": "FIXED_PRICING",
								"price_money": map[string]any{"amount": 1000, "currency": "USD"},
							}},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{
		SDK:                 squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL)),
		WholesaleCategoryID: categoryID,
	}
	items, err := c.GetWholesaleCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestGetWholesaleCatalog_RequiresCategoryID(t *testing.T) {
	c := &Client{}
	_, err := c.GetWholesaleCatalog(context.Background())
	if err == nil {
		t.Fatal("expected error when wholesale category ID is empty")
	}
}

func TestGetWholesaleCatalog_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{
		SDK:                 squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL)),
		WholesaleCategoryID: "configured-category-id",
	}
	_, err := c.GetWholesaleCatalog(context.Background())
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}
