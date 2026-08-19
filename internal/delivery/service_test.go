package delivery

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestPolicyAppliesDeliveryRules(t *testing.T) {
	policy := Policy{MaxMiles: 30, FreeMinimumCents: 10000, RateCentsPerMile: 50}
	options := policy.Options()
	if options.MaxMiles != 30 || options.FreeMinimumCents != 10000 || options.RateCentsPerMile != 50 {
		t.Fatalf("options = %+v", options)
	}

	tests := []struct {
		name          string
		distanceMiles float64
		subtotalCents int64
		wantFee       int64
		wantErr       error
	}{
		{name: "rounds exact mile", distanceMiles: 12, subtotalCents: 9999, wantFee: 600},
		{name: "rounds partial mile up", distanceMiles: 12.01, subtotalCents: 9999, wantFee: 650},
		{name: "free at minimum", distanceMiles: 12.01, subtotalCents: 10000, wantFee: 0},
		{name: "allows boundary", distanceMiles: 30, subtotalCents: 5000, wantFee: 1500},
		{name: "rejects over boundary", distanceMiles: 30.01, subtotalCents: 5000, wantErr: ErrOutsideDeliveryArea},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote, err := policy.Calculate(tt.distanceMiles*metersPerMile, tt.subtotalCents, "USD")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if quote.FeeCents != tt.wantFee {
				t.Fatalf("fee = %d, want %d", quote.FeeCents, tt.wantFee)
			}
		})
	}
}

type fakeOriginProvider struct {
	origin Origin
	err    error
}

func (f *fakeOriginProvider) DeliveryOrigin(context.Context, string) (Origin, error) {
	return f.origin, f.err
}

type fakeRouter struct {
	geocoded Address
	distance float64
	err      error
}

func (f *fakeRouter) Geocode(_ context.Context, address Address) (Coordinates, error) {
	f.geocoded = address
	return Coordinates{Longitude: -77.4, Latitude: 37.5}, f.err
}

func (f *fakeRouter) DrivingDistanceMeters(context.Context, Coordinates, Coordinates) (float64, error) {
	return f.distance, f.err
}

func TestServiceQuotesDrivingDistanceFromSquareLocationCoordinates(t *testing.T) {
	router := &fakeRouter{distance: 12.2 * metersPerMile}
	service := NewService(
		"LOCATION1",
		&fakeOriginProvider{origin: Origin{Coordinates: &Coordinates{Longitude: -77.5, Latitude: 37.6}}},
		router,
		Policy{MaxMiles: 30, FreeMinimumCents: 10000, RateCentsPerMile: 50},
	)

	quote, err := service.Quote(context.Background(), Address{
		Line1: "123 Main St", City: "Richmond", State: "VA", PostalCode: "23220", Country: "US",
	}, 8400, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(quote.DistanceMiles-12.2) > 0.001 || quote.FeeCents != 650 {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	if router.geocoded.Line1 != "123 Main St" {
		t.Fatalf("destination was not geocoded: %+v", router.geocoded)
	}
}

func TestServiceGeocodesOriginWhenSquareHasNoCoordinates(t *testing.T) {
	router := &fakeRouter{distance: 5 * metersPerMile}
	originAddress := Address{Line1: "1 Coffee Way", City: "Richmond", State: "VA", PostalCode: "23220", Country: "US"}
	service := NewService(
		"LOCATION1",
		&fakeOriginProvider{origin: Origin{Address: originAddress}},
		router,
		Policy{MaxMiles: 30, FreeMinimumCents: 10000, RateCentsPerMile: 50},
	)

	if _, err := service.Quote(context.Background(), Address{Line1: "123 Main St", City: "Richmond", State: "VA", PostalCode: "23220", Country: "US"}, 5000, "USD"); err != nil {
		t.Fatal(err)
	}
	if router.geocoded.Line1 != "123 Main St" {
		t.Fatalf("last geocoded address = %+v", router.geocoded)
	}
}
