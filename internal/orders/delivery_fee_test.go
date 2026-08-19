package orders

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reverendheat/gccr_invoice/internal/square"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func TestSubmitToSquareAddsSnapshottedDeliveryFee(t *testing.T) {
	app, customer := newOrderTestApp(t)
	order, err := Create(app, customer.Id, "", []LineItem{{
		VariationID: "VAR1", Name: "Coffee", Quantity: 2, UnitPriceCents: 1250, Currency: "USD",
	}}, Fulfillment{
		Method: FulfillmentDelivery, RecipientName: "Customer", RecipientPhone: "555-123-4567",
		AddressLine1: "123 Main St", City: "Richmond", State: "VA", PostalCode: "23220", Country: "US",
		DistanceMeters: 19634, DistanceMiles: 12.2, FeeCents: 650, Currency: "USD", PricingRule: "local-delivery-v1",
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		squareOrder := body["order"].(map[string]any)
		lineItems := squareOrder["line_items"].([]any)
		if len(lineItems) != 2 {
			t.Fatalf("line item count = %d, want 2", len(lineItems))
		}
		deliveryLine := lineItems[1].(map[string]any)
		if deliveryLine["name"] != "Local delivery" || deliveryLine["quantity"] != "1" {
			t.Fatalf("delivery line = %#v", deliveryLine)
		}
		price := deliveryLine["base_price_money"].(map[string]any)
		if price["amount"] != float64(650) || price["currency"] != "USD" {
			t.Fatalf("delivery price = %#v", price)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"order": map[string]any{"id": "SQORDER1"}})
	}))
	defer srv.Close()

	sq := &square.Client{SDK: squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL))}
	if _, err := SubmitToSquare(context.Background(), app, sq, "HILLTOP", "SQCUSTOMER1", order); err != nil {
		t.Fatal(err)
	}
}
