package scheduler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/reverendheat/gccr_invoice/internal/square"
	_ "github.com/reverendheat/gccr_invoice/pb_migrations"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func TestProcessOnePricesForScheduleOwner(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir(), HideStartBanner: true})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAppMigrations(); err != nil {
		t.Fatal(err)
	}

	customers, err := app.FindCollectionByNameOrId("customers")
	if err != nil {
		t.Fatal(err)
	}
	customer := core.NewRecord(customers)
	customer.SetEmail("owner@example.com")
	customer.SetPassword("test-password-12345")
	customer.Set("name", "Schedule Owner")
	customer.Set("squareCustomerId", "SQ_SCHEDULE_OWNER")
	if err := app.Save(customer); err != nil {
		t.Fatal(err)
	}

	schedules, err := app.FindCollectionByNameOrId("scheduledOrders")
	if err != nil {
		t.Fatal(err)
	}
	schedule := core.NewRecord(schedules)
	schedule.Set("customer", customer.Id)
	schedule.Set("frequency", "weekly")
	schedule.Set("lineItems", []map[string]any{{"variation_id": "VAR1", "quantity": 2}})
	schedule.Set("fulfillment", map[string]any{"method": "pickup"})
	schedule.Set("next_run_at", "2026-01-01 00:00:00.000Z")
	schedule.Set("active", true)
	if err := app.Save(schedule); err != nil {
		t.Fatal(err)
	}

	var requestedCustomerIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/v2/customers/") {
			customerID := strings.TrimPrefix(r.URL.Path, "/v2/customers/")
			requestedCustomerIDs = append(requestedCustomerIDs, customerID)
			_ = json.NewEncoder(w).Encode(map[string]any{"customer": map[string]any{
				"id": customerID, "group_ids": []string{"GROCERY_GROUP"},
			}})
			return
		}
		if r.URL.Path == "/v2/catalog/search-catalog-items" {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
				"id": "ITEM1", "type": "ITEM",
				"custom_attribute_values": map[string]any{
					"prefixed:grocery": map[string]any{
						"custom_attribute_definition_id": "GROCERY_ATTRIBUTE", "type": "BOOLEAN", "boolean_value": true,
					},
					"prefixed:cafe": map[string]any{
						"custom_attribute_definition_id": "CAFE_ATTRIBUTE", "type": "BOOLEAN", "boolean_value": false,
					},
				},
				"item_data": map[string]any{
					"name": "Coffee",
					"variations": []map[string]any{{
						"id": "VAR1", "type": "ITEM_VARIATION",
						"item_variation_data": map[string]any{
							"name": "Default", "pricing_type": "FIXED_PRICING",
							"price_money": map[string]any{"amount": 1000, "currency": "USD"},
						},
					}},
				},
			}}})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	sq := &square.Client{
		SDK:                                   squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL)),
		WholesaleCategoryID:                   "WHOLESALE",
		WholesaleGroceryGroupID:               "GROCERY_GROUP",
		WholesaleCafeRestaurantGroupID:        "CAFE_GROUP",
		WholesaleGroceryAttributeID:           "GROCERY_ATTRIBUTE",
		WholesaleCafeRestaurantAttributeID:    "CAFE_ATTRIBUTE",
		WholesaleCustomerAllowlistAttributeID: "ALLOWLIST_ATTRIBUTE",
	}
	if err := processOne(app, sq, nil, schedule); err != nil {
		t.Fatal(err)
	}
	if len(requestedCustomerIDs) != 1 || requestedCustomerIDs[0] != "SQ_SCHEDULE_OWNER" {
		t.Fatalf("Square customer requests = %v, want [SQ_SCHEDULE_OWNER]", requestedCustomerIDs)
	}

	created, err := app.FindRecordsByFilter("orders", "customer = {:customer}", "", 10, 0, map[string]any{"customer": customer.Id})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("created orders = %d, want 1", len(created))
	}
}
