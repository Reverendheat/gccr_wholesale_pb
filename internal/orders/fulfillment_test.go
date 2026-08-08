package orders

import "testing"

func TestNormalizeFulfillmentDefaultsToPickup(t *testing.T) {
	got, err := NormalizeFulfillment(Fulfillment{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != FulfillmentPickup {
		t.Fatalf("method = %q, want %q", got.Method, FulfillmentPickup)
	}
}

func TestNormalizeFulfillmentAcceptsUSDelivery(t *testing.T) {
	got, err := NormalizeFulfillment(Fulfillment{
		Method:         FulfillmentDelivery,
		RecipientName:  " Jane Smith ",
		RecipientPhone: " 555-0100 ",
		AddressLine1:   " 123 Main St ",
		City:           " Richmond ",
		State:          " va ",
		PostalCode:     " 23220 ",
		Country:        " us ",
		Instructions:   " Rear entrance ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.RecipientName != "Jane Smith" || got.State != "VA" || got.Country != "US" {
		t.Fatalf("delivery was not normalized: %+v", got)
	}
}

func TestNormalizeFulfillmentRejectsIncompleteDelivery(t *testing.T) {
	tests := []struct {
		name        string
		fulfillment Fulfillment
	}{
		{"missing recipient", Fulfillment{Method: FulfillmentDelivery, RecipientPhone: "555-0100", AddressLine1: "123 Main", City: "Richmond", State: "VA", PostalCode: "23220", Country: "US"}},
		{"missing phone", Fulfillment{Method: FulfillmentDelivery, RecipientName: "Jane", AddressLine1: "123 Main", City: "Richmond", State: "VA", PostalCode: "23220", Country: "US"}},
		{"invalid state format", Fulfillment{Method: FulfillmentDelivery, RecipientName: "Jane", RecipientPhone: "555-0100", AddressLine1: "123 Main", City: "Richmond", State: "Virginia", PostalCode: "23220", Country: "US"}},
		{"unknown state code", Fulfillment{Method: FulfillmentDelivery, RecipientName: "Jane", RecipientPhone: "555-0100", AddressLine1: "123 Main", City: "Richmond", State: "ZZ", PostalCode: "23220", Country: "US"}},
		{"invalid postal code", Fulfillment{Method: FulfillmentDelivery, RecipientName: "Jane", RecipientPhone: "555-0100", AddressLine1: "123 Main", City: "Richmond", State: "VA", PostalCode: "ABC", Country: "US"}},
		{"non-US country", Fulfillment{Method: FulfillmentDelivery, RecipientName: "Jane", RecipientPhone: "555-0100", AddressLine1: "123 Main", City: "Richmond", State: "VA", PostalCode: "23220", Country: "CA"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NormalizeFulfillment(tt.fulfillment); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
