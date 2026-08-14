package orders

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/reverendheat/gccr_invoice/internal/square"
	_ "github.com/reverendheat/gccr_invoice/pb_migrations"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func TestLockPricesSnapshotsSubmissionPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
			"id": "ITEM1", "type": "ITEM",
			"item_data": map[string]any{
				"name": "Wholesale Coffee",
				"variations": []map[string]any{{
					"id": "VAR1", "type": "ITEM_VARIATION",
					"item_variation_data": map[string]any{
						"name": "5 lb", "pricing_type": "FIXED_PRICING",
						"price_money": map[string]any{"amount": 1000, "currency": "USD"},
						"location_overrides": []map[string]any{{
							"location_id": "HILLTOP",
							"price_money": map[string]any{"amount": 1250, "currency": "USD"},
						}},
					},
				}},
			},
		}}})
	}))
	defer srv.Close()

	sq := &square.Client{
		SDK:                 squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL)),
		WholesaleCategoryID: "WHOLESALE",
	}
	items, err := LockPrices(context.Background(), sq, "HILLTOP", []LineItem{{VariationID: "VAR1", Quantity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "Wholesale Coffee - 5 lb" || items[0].UnitPriceCents != 1250 || items[0].Currency != "USD" {
		t.Fatalf("unexpected locked item: %+v", items)
	}
	if _, err := LockPrices(context.Background(), sq, "HILLTOP", []LineItem{{VariationID: "NOT_WHOLESALE", Quantity: 1}}); err == nil {
		t.Fatal("expected unavailable variation error")
	}
}

func TestCreateStoresLocalPricedOrderWithoutSquareOrder(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := Create(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 2, UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "note")
	if err != nil {
		t.Fatal(err)
	}
	if order.GetString("squareOrderId") != "" {
		t.Fatalf("squareOrderId = %q, want empty", order.GetString("squareOrderId"))
	}
	var items []LineItem
	if err := order.UnmarshalJSONField("lineItems", &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].UnitPriceCents != 1250 || items[0].Name != "Coffee" {
		t.Fatalf("unexpected snapshot: %+v", items)
	}
}

func TestSubmitToSquareUsesLockedPriceAndPersistsSquareID(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := Create(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 2, Note: "fine", UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		squareOrder := body["order"].(map[string]any)
		lineItem := squareOrder["line_items"].([]any)[0].(map[string]any)
		price := lineItem["base_price_money"].(map[string]any)
		if price["amount"] != float64(1250) || price["currency"] != "USD" {
			t.Fatalf("Square price = %v, want locked USD 1250", price)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"order": map[string]any{
			"id": "SQORDER1", "location_id": "HILLTOP",
		}})
	}))
	defer srv.Close()

	sq := &square.Client{SDK: squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL))}
	got, err := SubmitToSquare(context.Background(), app, sq, "HILLTOP", "SQCUSTOMER1", order)
	if err != nil {
		t.Fatal(err)
	}
	if got != "SQORDER1" {
		t.Fatalf("Square order ID = %q", got)
	}
	order, _ = app.FindRecordById("orders", order.Id)
	if order.GetString("squareOrderId") != "SQORDER1" {
		t.Fatalf("persisted squareOrderId = %q", order.GetString("squareOrderId"))
	}
}

func newOrderTestApp(t *testing.T) (*pocketbase.PocketBase, *core.Record) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir(), HideStartBanner: true})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	if err := app.RunAppMigrations(); err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId("customers")
	if err != nil {
		t.Fatal(err)
	}
	customer := core.NewRecord(collection)
	customer.SetEmail("customer@example.com")
	customer.SetPassword("test-password-12345")
	customer.Set("name", "Customer")
	customer.Set("squareCustomerId", "SQCUSTOMER1")
	if err := app.Save(customer); err != nil {
		t.Fatal(err)
	}
	return app, customer
}
