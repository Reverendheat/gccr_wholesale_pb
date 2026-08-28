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
		switch r.URL.Path {
		case "/v2/customers/SQUARE_CUSTOMER":
			_ = json.NewEncoder(w).Encode(map[string]any{"customer": map[string]any{
				"id": "SQUARE_CUSTOMER", "group_ids": []string{"GROCERY_GROUP"},
			}})
		case "/v2/catalog/search-catalog-items":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{
				{
					"id": "ITEM1", "type": "ITEM",
					"custom_attribute_values": map[string]any{
						"prefixed:grocery": map[string]any{
							"custom_attribute_definition_id": "GROCERY_ATTRIBUTE",
							"type":                           "BOOLEAN", "boolean_value": true,
						},
						"prefixed:cafe": map[string]any{
							"custom_attribute_definition_id": "CAFE_ATTRIBUTE",
							"type":                           "BOOLEAN", "boolean_value": false,
						},
					},
					"item_data": map[string]any{
						"name": "Wholesale Coffee",
						"modifier_list_info": []map[string]any{{
							"modifier_list_id": "GROUND_LIST", "enabled": true,
						}},
						"variations": []map[string]any{{
							"id": "VAR1", "type": "ITEM_VARIATION",
							"item_variation_data": map[string]any{
								"name": "5 lb", "pricing_type": "FIXED_PRICING",
								"price_money": map[string]any{"amount": 1000, "currency": "USD"},
							},
						}},
					},
				},
				{
					"id": "HIDDEN_ITEM", "type": "ITEM",
					"custom_attribute_values": map[string]any{
						"prefixed:grocery": map[string]any{
							"custom_attribute_definition_id": "GROCERY_ATTRIBUTE",
							"type":                           "BOOLEAN", "boolean_value": false,
						},
						"prefixed:cafe": map[string]any{
							"custom_attribute_definition_id": "CAFE_ATTRIBUTE",
							"type":                           "BOOLEAN", "boolean_value": true,
						},
					},
					"item_data": map[string]any{
						"name": "Hidden Coffee",
						"variations": []map[string]any{{
							"id": "HIDDEN_VAR", "type": "ITEM_VARIATION",
							"item_variation_data": map[string]any{
								"name": "5 lb", "pricing_type": "FIXED_PRICING",
								"price_money": map[string]any{"amount": 900, "currency": "USD"},
							},
						}},
					},
				},
			}})
		case "/v2/catalog/batch-retrieve":
			_ = json.NewEncoder(w).Encode(map[string]any{"objects": []map[string]any{{
				"id": "GROUND_LIST", "type": "MODIFIER_LIST",
				"modifier_list_data": map[string]any{
					"name": "Ground", "selection_type": "SINGLE",
					"modifiers": []map[string]any{{
						"id": "DRIP", "type": "MODIFIER",
						"modifier_data": map[string]any{
							"name":        "Drip",
							"price_money": map[string]any{"amount": 0, "currency": "USD"},
						},
					}},
				},
			}}})
		default:
			http.NotFound(w, r)
		}
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
	items, err := LockPrices(context.Background(), sq, "SQUARE_CUSTOMER", []LineItem{{
		VariationID: "VAR1", GrindModifierID: "DRIP", Quantity: 2,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "Wholesale Coffee - 5 lb" ||
		items[0].UnitPriceCents != 1000 || items[0].Currency != "USD" ||
		items[0].Grind != "Drip" || items[0].GrindModifierID != "DRIP" {
		t.Fatalf("unexpected locked item: %+v", items)
	}
	wholeBean, err := LockPrices(context.Background(), sq, "SQUARE_CUSTOMER", []LineItem{{
		VariationID: "VAR1", Quantity: 1,
	}})
	if err != nil || len(wholeBean) != 1 || wholeBean[0].Grind != "Whole Bean" {
		t.Fatalf("whole bean lock = %+v, err = %v", wholeBean, err)
	}
	_, err = LockPrices(context.Background(), sq, "SQUARE_CUSTOMER", []LineItem{{
		VariationID: "VAR1", GrindModifierID: "UNKNOWN", Quantity: 1,
	}})
	if err == nil || err.Error() != `grind modifier "UNKNOWN" is unavailable for variation "VAR1"` {
		t.Fatalf("unknown grind error = %v", err)
	}
	_, err = LockPrices(context.Background(), sq, "SQUARE_CUSTOMER", []LineItem{{VariationID: "HIDDEN_VAR", Quantity: 1}})
	if err == nil || err.Error() != `variation "HIDDEN_VAR" is unavailable in the wholesale catalog` {
		t.Fatalf("hidden variation error = %v", err)
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

func TestCreateWithPlacementStoresStaffAudit(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := CreateWithPlacement(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 1, UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "customer note", Placement{
		Actor: Actor{Type: ActorStaff, ID: "STAFF1", Name: "Jane Staff"},
		Note:  "Phone order",
	})
	if err != nil {
		t.Fatal(err)
	}
	var actor Actor
	if err := order.UnmarshalJSONField("placedBy", &actor); err != nil {
		t.Fatal(err)
	}
	if actor.Type != ActorStaff || actor.ID != "STAFF1" || actor.Name != "Jane Staff" {
		t.Fatalf("placedBy = %+v", actor)
	}
	if order.GetString("placementNote") != "Phone order" {
		t.Fatalf("placementNote = %q", order.GetString("placementNote"))
	}
}

func TestUpdatePendingReplacesEditableOrderSnapshot(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := Create(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 1, UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "old note")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := UpdatePending(app, order, []LineItem{{
		VariationID: "VAR2", Name: "Tea", Quantity: 3, UnitPriceCents: 800, Currency: "USD",
	}}, Fulfillment{
		Method: FulfillmentDelivery, RecipientName: "Customer", RecipientPhone: "555-123-4567",
		AddressLine1: "123 Main St", City: "Phoenix", State: "AZ", PostalCode: "85001", Country: "US",
	}, "new note")
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetString("notes") != "new note" {
		t.Fatalf("notes = %q", updated.GetString("notes"))
	}
	var items []LineItem
	if err := updated.UnmarshalJSONField("lineItems", &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].VariationID != "VAR2" || items[0].Quantity != 3 || items[0].UnitPriceCents != 800 {
		t.Fatalf("updated items = %+v", items)
	}
	var fulfillment Fulfillment
	if err := updated.UnmarshalJSONField("fulfillment", &fulfillment); err != nil {
		t.Fatal(err)
	}
	if fulfillment.Method != FulfillmentDelivery || fulfillment.AddressLine1 != "123 Main St" {
		t.Fatalf("updated fulfillment = %+v", fulfillment)
	}
}

func TestUpdateByStaffAllowsConfirmedPreSquareOrderAndAudits(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := CreateWithPlacement(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 1, UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "old note", Placement{
		Actor: Actor{Type: ActorCustomer, ID: customer.Id, Name: "Customer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	order.Set("status", "confirmed")
	if err := app.Save(order); err != nil {
		t.Fatal(err)
	}

	updated, err := UpdateByStaff(app, order, []LineItem{{
		VariationID: "VAR2", Name: "Cold Brew Growler", Quantity: 2, UnitPriceCents: 1800, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "add growlers", EditAudit{
		Actor:  Actor{Type: ActorStaff, ID: "STAFF1", Name: "Jane Staff"},
		Reason: "Customer requested one more growler", EditedAt: "2026-08-20T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetString("status") != "confirmed" || updated.GetString("notes") != "add growlers" {
		t.Fatalf("updated order = status %q notes %q", updated.GetString("status"), updated.GetString("notes"))
	}
	var history []EditAudit
	if err := updated.UnmarshalJSONField("editHistory", &history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Actor.ID != "STAFF1" || history[0].Reason != "Customer requested one more growler" {
		t.Fatalf("edit history = %+v", history)
	}
}

func TestUpdateByStaffRejectsSquareSubmittedOrder(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := Create(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 1, UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "")
	if err != nil {
		t.Fatal(err)
	}
	order.Set("squareOrderId", "SQ1")
	if err := app.Save(order); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateByStaff(app, order, []LineItem{{VariationID: "VAR1", Quantity: 2, UnitPriceCents: 1250, Currency: "USD"}}, Fulfillment{Method: FulfillmentPickup}, "", EditAudit{Actor: Actor{Type: ActorStaff, ID: "STAFF1", Name: "Jane"}, Reason: "change"}); err == nil {
		t.Fatal("expected Square-submitted order edit to fail")
	}
}

func TestUpdatePendingRejectsNonPendingOrder(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := Create(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 1, UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "old note")
	if err != nil {
		t.Fatal(err)
	}
	order.Set("status", "confirmed")
	if err := app.Save(order); err != nil {
		t.Fatal(err)
	}

	_, err = UpdatePending(app, order, []LineItem{{
		VariationID: "VAR2", Name: "Tea", Quantity: 3, UnitPriceCents: 800, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "new note")
	if err == nil {
		t.Fatal("expected confirmed order update to fail")
	}

	unchanged, err := app.FindRecordById("orders", order.Id)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.GetString("notes") != "old note" {
		t.Fatalf("notes changed to %q", unchanged.GetString("notes"))
	}
}

func TestUpdatePendingRejectsOrderAlreadySubmittedToSquare(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := Create(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 1, UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "old note")
	if err != nil {
		t.Fatal(err)
	}
	order.Set("squareOrderId", "SQORDER1")
	if err := app.Save(order); err != nil {
		t.Fatal(err)
	}

	_, err = UpdatePending(app, order, []LineItem{{
		VariationID: "VAR2", Name: "Tea", Quantity: 3, UnitPriceCents: 800, Currency: "USD",
	}}, Fulfillment{Method: FulfillmentPickup}, "new note")
	if err == nil {
		t.Fatal("expected Square-submitted order update to fail")
	}
}

func TestSubmitToSquareUsesLockedPriceAndPersistsSquareID(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := Create(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 2, Note: "fine",
		Grind: "Drip", GrindModifierID: "DRIP", UnitPriceCents: 1250, Currency: "USD",
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
		modifiers := lineItem["modifiers"].([]any)
		if len(modifiers) != 1 || modifiers[0].(map[string]any)["catalog_object_id"] != "DRIP" {
			t.Fatalf("Square modifiers = %v, want DRIP", modifiers)
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
