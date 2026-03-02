package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateOrder_Success(t *testing.T) {
	wantOrder := Order{ID: "ORDER123", LocationID: "LOC1", CustomerID: "CUST1"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/orders" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body createOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if body.Order.LocationID != "LOC1" {
			t.Errorf("expected location LOC1, got %s", body.Order.LocationID)
		}
		if body.Order.CustomerID != "CUST1" {
			t.Errorf("expected customer CUST1, got %s", body.Order.CustomerID)
		}
		if body.IdempotencyKey == "" {
			t.Error("expected non-empty idempotency key")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(createOrderResponse{Order: wantOrder})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, accessToken: "tok", http: &http.Client{}}
	items := []OrderLineItem{{CatalogObjectID: "VAR1", Quantity: "3"}}

	got, err := c.CreateOrder(context.Background(), "LOC1", "CUST1", items, "pb-order-1", "idem-key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != wantOrder.ID {
		t.Errorf("expected order ID %q, got %q", wantOrder.ID, got.ID)
	}
}

func TestCreateOrder_MissingLocationID(t *testing.T) {
	c := &Client{baseURL: "http://unused", accessToken: "tok", http: &http.Client{}}
	_, err := c.CreateOrder(context.Background(), "", "CUST1", []OrderLineItem{{CatalogObjectID: "V1", Quantity: "1"}}, "", "key")
	if err == nil {
		t.Fatal("expected error for missing locationID")
	}
}

func TestCreateOrder_EmptyLineItems(t *testing.T) {
	c := &Client{baseURL: "http://unused", accessToken: "tok", http: &http.Client{}}
	_, err := c.CreateOrder(context.Background(), "LOC1", "CUST1", nil, "", "key")
	if err == nil {
		t.Fatal("expected error for empty line items")
	}
}
