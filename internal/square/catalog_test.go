package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWholesaleCatalog_Success(t *testing.T) {
	want := []CatalogItem{
		{ID: "ITEM1", Type: "ITEM"},
		{ID: "ITEM2", Type: "ITEM"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/catalog/search-catalog-items" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify the request body contains the wholesale category ID
		var body searchCatalogItemsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("could not decode request body: %v", err)
		}
		if len(body.CategoryIDs) == 0 || body.CategoryIDs[0] != wholesaleCategoryID {
			t.Errorf("expected wholesale category ID in request")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(searchCatalogItemsResponse{Items: want})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, accessToken: "tok", http: &http.Client{}}
	items, err := c.GetWholesaleCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != len(want) {
		t.Errorf("expected %d items, got %d", len(want), len(items))
	}
}

func TestGetWholesaleCatalog_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, accessToken: "tok", http: &http.Client{}}
	_, err := c.GetWholesaleCatalog(context.Background())
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}
