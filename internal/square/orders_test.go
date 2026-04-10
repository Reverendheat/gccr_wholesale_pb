package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	squaresdk "github.com/square/square-go-sdk/v3"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func TestCreateOrder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/orders" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		order, _ := body["order"].(map[string]any)
		if order["location_id"] != "LOC1" {
			t.Errorf("expected location LOC1, got %v", order["location_id"])
		}
		if order["customer_id"] != "CUST1" {
			t.Errorf("expected customer CUST1, got %v", order["customer_id"])
		}
		if body["idempotency_key"] == "" {
			t.Error("expected non-empty idempotency key")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":          "ORDER123",
				"location_id": "LOC1",
				"customer_id": "CUST1",
			},
		})
	}))
	defer srv.Close()

	c := &Client{SDK: squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL))}
	items := []*squaresdk.OrderLineItem{{CatalogObjectID: squaresdk.String("VAR1"), Quantity: "3"}}

	got, err := c.CreateOrder(context.Background(), "LOC1", "CUST1", items, "pb-order-1", "idem-key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == nil || *got.ID != "ORDER123" {
		t.Errorf("expected order ID %q, got %v", "ORDER123", got.ID)
	}
}

func TestCreateOrder_MissingLocationID(t *testing.T) {
	c := &Client{SDK: squareclient.NewClient(option.WithToken("tok"))}
	_, err := c.CreateOrder(context.Background(), "", "CUST1", []*squaresdk.OrderLineItem{{CatalogObjectID: squaresdk.String("V1"), Quantity: "1"}}, "", "key")
	if err == nil {
		t.Fatal("expected error for missing locationID")
	}
}

func TestCreateOrder_EmptyLineItems(t *testing.T) {
	c := &Client{SDK: squareclient.NewClient(option.WithToken("tok"))}
	_, err := c.CreateOrder(context.Background(), "LOC1", "CUST1", nil, "", "key")
	if err == nil {
		t.Fatal("expected error for empty line items")
	}
}
