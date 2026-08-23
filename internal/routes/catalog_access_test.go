package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/reverendheat/gccr_invoice/internal/orders"
	"github.com/reverendheat/gccr_invoice/internal/square"
	_ "github.com/reverendheat/gccr_invoice/pb_migrations"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func newCatalogAccessTestApp(t *testing.T) (*pocketbase.PocketBase, *core.Record, *core.Record, *core.Record, *core.Record) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir(), HideStartBanner: true})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	if err := app.RunAppMigrations(); err != nil {
		t.Fatal(err)
	}

	customers, err := app.FindCollectionByNameOrId("customers")
	if err != nil {
		t.Fatal(err)
	}
	newCustomer := func(email, squareID string) *core.Record {
		record := core.NewRecord(customers)
		record.SetEmail(email)
		record.SetPassword("test-password-12345")
		record.Set("name", email)
		record.Set("squareCustomerId", squareID)
		save := app.Save
		if squareID == "" {
			save = app.SaveNoValidate
		}
		if err := save(record); err != nil {
			t.Fatal(err)
		}
		return record
	}
	self := newCustomer("self@example.com", "SQ_SELF")
	target := newCustomer("target@example.com", "SQ_TARGET")
	unlinked := newCustomer("unlinked@example.com", "")

	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	staff := core.NewRecord(users)
	staff.SetEmail("staff@example.com")
	staff.SetPassword("test-password-12345")
	staff.Set("name", "Staff")
	if err := app.Save(staff); err != nil {
		t.Fatal(err)
	}
	return app, self, target, unlinked, staff
}

func newCatalogAccessSquare(t *testing.T) (*square.Client, func() []string, func()) {
	t.Helper()
	var customerIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/v2/customers/") {
			customerID := strings.TrimPrefix(r.URL.Path, "/v2/customers/")
			customerIDs = append(customerIDs, customerID)
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
	t.Cleanup(srv.Close)

	client := &square.Client{
		SDK:                                   squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL)),
		WholesaleCategoryID:                   "WHOLESALE",
		WholesaleGroceryGroupID:               "GROCERY_GROUP",
		WholesaleCafeRestaurantGroupID:        "CAFE_GROUP",
		WholesaleGroceryAttributeID:           "GROCERY_ATTRIBUTE",
		WholesaleCafeRestaurantAttributeID:    "CAFE_ATTRIBUTE",
		WholesaleCustomerAllowlistAttributeID: "ALLOWLIST_ATTRIBUTE",
	}
	return client, func() []string { return append([]string(nil), customerIDs...) }, func() { customerIDs = nil }
}

func invokeCatalogAccessHandler(
	t *testing.T,
	app core.App,
	auth *core.Record,
	method string,
	url string,
	body any,
	pathValues map[string]string,
	handler func(*core.RequestEvent) error,
) (*httptest.ResponseRecorder, error) {
	t.Helper()
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, url, requestBody)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range pathValues {
		req.SetPathValue(key, value)
	}
	recorder := httptest.NewRecorder()
	event := &core.RequestEvent{
		App:   app,
		Auth:  auth,
		Event: router.Event{Request: req, Response: recorder},
	}
	return recorder, handler(event)
}

func requireSquareCustomerRequest(t *testing.T, got []string, want string) {
	t.Helper()
	if len(got) != 1 || got[0] != want {
		t.Fatalf("Square customer requests = %v, want [%s]", got, want)
	}
}

func TestCatalogAccessTargetsAuthenticatedOrStaffCustomer(t *testing.T) {
	app, self, target, unlinked, staff := newCatalogAccessTestApp(t)
	sq, requests, reset := newCatalogAccessSquare(t)

	_, err := invokeCatalogAccessHandler(t, app, self, http.MethodGet, "/catalog?customer_id="+target.Id, nil, nil, handleCatalog(sq))
	if err != nil {
		t.Fatal(err)
	}
	requireSquareCustomerRequest(t, requests(), "SQ_SELF")
	reset()

	_, err = invokeCatalogAccessHandler(t, app, staff, http.MethodGet, "/catalog", nil, nil, handleCatalog(sq))
	apiErr := router.ToApiError(err)
	if apiErr.Status != http.StatusBadRequest || apiErr.Message != "Customer_id is required for staff catalog access." {
		t.Fatalf("missing staff target error = %#v", apiErr)
	}
	if len(requests()) != 0 {
		t.Fatalf("Square called without staff target: %v", requests())
	}

	_, err = invokeCatalogAccessHandler(t, app, staff, http.MethodGet, "/catalog?customer_id="+target.Id, nil, nil, handleCatalog(sq))
	if err != nil {
		t.Fatal(err)
	}
	requireSquareCustomerRequest(t, requests(), "SQ_TARGET")
	reset()

	_, err = invokeCatalogAccessHandler(t, app, staff, http.MethodGet, "/catalog?customer_id="+unlinked.Id, nil, nil, handleCatalog(sq))
	apiErr = router.ToApiError(err)
	if apiErr.Status != http.StatusBadRequest || apiErr.Message != "Customer is not linked to Square." {
		t.Fatalf("unlinked staff target error = %#v", apiErr)
	}
}

func TestFulfillmentQuoteUsesSelfOrStaffTarget(t *testing.T) {
	app, self, target, _, staff := newCatalogAccessTestApp(t)
	sq, requests, reset := newCatalogAccessSquare(t)
	body := map[string]any{
		"customer_id": target.Id,
		"lineItems":   []map[string]any{{"variation_id": "VAR1", "quantity": 1}},
		"fulfillment": map[string]any{"method": "pickup"},
	}

	_, err := invokeCatalogAccessHandler(t, app, self, http.MethodPost, "/quote", body, nil, handleFulfillmentQuote(sq, nil))
	if err != nil {
		t.Fatal(err)
	}
	requireSquareCustomerRequest(t, requests(), "SQ_SELF")
	reset()

	_, err = invokeCatalogAccessHandler(t, app, staff, http.MethodPost, "/quote", body, nil, handleFulfillmentQuote(sq, nil))
	if err != nil {
		t.Fatal(err)
	}
	requireSquareCustomerRequest(t, requests(), "SQ_TARGET")
}

func TestOrderEditsUseOrderOwnerSquareCustomer(t *testing.T) {
	app, self, target, _, staff := newCatalogAccessTestApp(t)
	sq, requests, reset := newCatalogAccessSquare(t)
	selfOrder, err := orders.CreateWithPlacement(
		app, self.Id, "", []orders.LineItem{{VariationID: "VAR1", Name: "Coffee", Quantity: 1, UnitPriceCents: 1000, Currency: "USD"}},
		orders.Fulfillment{Method: orders.FulfillmentPickup}, "", orders.Placement{Actor: orders.Actor{Type: orders.ActorCustomer, ID: self.Id, Name: "Self Customer"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	customerEditBody := map[string]any{
		"lineItems":   []map[string]any{{"variation_id": "VAR1", "quantity": 2}},
		"fulfillment": map[string]any{"method": "pickup"},
	}
	_, err = invokeCatalogAccessHandler(
		t, app, self, http.MethodPatch, "/orders/"+selfOrder.Id, customerEditBody,
		map[string]string{"id": selfOrder.Id}, handleUpdateOrder(sq, nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	requireSquareCustomerRequest(t, requests(), "SQ_SELF")
	reset()

	targetOrder, err := orders.CreateWithPlacement(
		app, target.Id, "", []orders.LineItem{{VariationID: "VAR1", Name: "Coffee", Quantity: 1, UnitPriceCents: 1000, Currency: "USD"}},
		orders.Fulfillment{Method: orders.FulfillmentPickup}, "", orders.Placement{Actor: orders.Actor{Type: orders.ActorCustomer, ID: target.Id, Name: "Target Customer"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	staffEditBody := map[string]any{
		"lineItems":   []map[string]any{{"variation_id": "VAR1", "quantity": 3}},
		"fulfillment": map[string]any{"method": "pickup"},
		"editReason":  "Customer requested a change",
	}
	_, err = invokeCatalogAccessHandler(
		t, app, staff, http.MethodPatch, "/orders/"+targetOrder.Id+"/staff", staffEditBody,
		map[string]string{"id": targetOrder.Id}, handleStaffUpdateOrder(sq, nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	requireSquareCustomerRequest(t, requests(), "SQ_TARGET")
}
