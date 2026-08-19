package orders

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/reverendheat/gccr_invoice/internal/delivery"
)

const (
	FulfillmentPickup   = "pickup"
	FulfillmentDelivery = "delivery"
)

var usPostalCodePattern = regexp.MustCompile(`^\d{5}(-\d{4})?$`)

var usStateCodes = map[string]bool{
	"AL": true, "AK": true, "AZ": true, "AR": true, "CA": true, "CO": true,
	"CT": true, "DE": true, "DC": true, "FL": true, "GA": true, "HI": true,
	"ID": true, "IL": true, "IN": true, "IA": true, "KS": true, "KY": true,
	"LA": true, "ME": true, "MD": true, "MA": true, "MI": true, "MN": true,
	"MS": true, "MO": true, "MT": true, "NE": true, "NV": true, "NH": true,
	"NJ": true, "NM": true, "NY": true, "NC": true, "ND": true, "OH": true,
	"OK": true, "OR": true, "PA": true, "RI": true, "SC": true, "SD": true,
	"TN": true, "TX": true, "UT": true, "VT": true, "VA": true, "WA": true,
	"WV": true, "WI": true, "WY": true,
}

type Fulfillment struct {
	Method         string  `json:"method"`
	RecipientName  string  `json:"recipient_name,omitempty"`
	RecipientPhone string  `json:"recipient_phone,omitempty"`
	AddressLine1   string  `json:"address_line_1,omitempty"`
	AddressLine2   string  `json:"address_line_2,omitempty"`
	City           string  `json:"city,omitempty"`
	State          string  `json:"state,omitempty"`
	PostalCode     string  `json:"postal_code,omitempty"`
	Country        string  `json:"country,omitempty"`
	Instructions   string  `json:"instructions,omitempty"`
	DistanceMeters float64 `json:"distance_meters,omitempty"`
	DistanceMiles  float64 `json:"distance_miles,omitempty"`
	FeeCents       int64   `json:"fee_cents,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	PricingRule    string  `json:"pricing_rule,omitempty"`
}

func NormalizeFulfillment(input Fulfillment) (Fulfillment, error) {
	input.Method = strings.ToLower(strings.TrimSpace(input.Method))
	if input.Method == "" {
		input.Method = FulfillmentPickup
	}
	if input.Method == FulfillmentPickup {
		return Fulfillment{Method: FulfillmentPickup}, nil
	}
	if input.Method != FulfillmentDelivery {
		return Fulfillment{}, fmt.Errorf("fulfillment method must be pickup or delivery")
	}

	input.RecipientName = strings.TrimSpace(input.RecipientName)
	input.RecipientPhone = strings.TrimSpace(input.RecipientPhone)
	input.AddressLine1 = strings.TrimSpace(input.AddressLine1)
	input.AddressLine2 = strings.TrimSpace(input.AddressLine2)
	input.City = strings.TrimSpace(input.City)
	input.State = strings.ToUpper(strings.TrimSpace(input.State))
	input.PostalCode = strings.TrimSpace(input.PostalCode)
	input.Country = strings.ToUpper(strings.TrimSpace(input.Country))
	input.Instructions = strings.TrimSpace(input.Instructions)
	// Distance and pricing are server-calculated. Never trust request values.
	input.DistanceMeters = 0
	input.DistanceMiles = 0
	input.FeeCents = 0
	input.Currency = ""
	input.PricingRule = ""

	if input.RecipientName == "" || input.RecipientPhone == "" || input.AddressLine1 == "" || input.City == "" || input.State == "" || input.PostalCode == "" {
		return Fulfillment{}, fmt.Errorf("delivery requires recipient name, phone, address, city, state, and postal code")
	}
	if !usStateCodes[input.State] {
		return Fulfillment{}, fmt.Errorf("delivery state must be a valid two-letter US state code")
	}
	if input.Country != "US" {
		return Fulfillment{}, fmt.Errorf("delivery country must be US")
	}
	if !usPostalCodePattern.MatchString(input.PostalCode) {
		return Fulfillment{}, fmt.Errorf("delivery postal code must be a valid US ZIP code")
	}

	return input, nil
}

// QuoteFulfillment normalizes client input and adds authoritative delivery pricing.
func QuoteFulfillment(
	ctx context.Context,
	quoter delivery.Quoter,
	input Fulfillment,
	items []LineItem,
) (Fulfillment, int64, error) {
	fulfillment, err := NormalizeFulfillment(input)
	if err != nil {
		return Fulfillment{}, 0, err
	}
	subtotal, currency, err := MerchandiseSubtotal(items)
	if err != nil {
		return Fulfillment{}, 0, err
	}
	if fulfillment.Method == FulfillmentPickup {
		return fulfillment, subtotal, nil
	}
	quote, err := quoter.Quote(ctx, delivery.Address{
		Line1:      fulfillment.AddressLine1,
		Line2:      fulfillment.AddressLine2,
		City:       fulfillment.City,
		State:      fulfillment.State,
		PostalCode: fulfillment.PostalCode,
		Country:    fulfillment.Country,
	}, subtotal, currency)
	if err != nil {
		return Fulfillment{}, 0, err
	}
	fulfillment.DistanceMeters = quote.DistanceMeters
	fulfillment.DistanceMiles = quote.DistanceMiles
	fulfillment.FeeCents = quote.FeeCents
	fulfillment.Currency = quote.Currency
	fulfillment.PricingRule = quote.PricingRule
	return fulfillment, subtotal, nil
}
