package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func TestDeliveryOriginUsesSquareLocationCoordinates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/locations/LOC1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"location": map[string]any{
			"id": "LOC1",
			"coordinates": map[string]any{"longitude": -77.5, "latitude": 37.6},
			"address": map[string]any{
				"address_line_1": "1 Coffee Way", "locality": "Richmond",
				"administrative_district_level_1": "VA", "postal_code": "23220", "country": "US",
			},
		}})
	}))
	defer srv.Close()

	client := &Client{SDK: squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL))}
	origin, err := client.DeliveryOrigin(context.Background(), "LOC1")
	if err != nil {
		t.Fatal(err)
	}
	if origin.Coordinates == nil || origin.Coordinates.Longitude != -77.5 || origin.Address.Line1 != "1 Coffee Way" {
		t.Fatalf("origin = %+v", origin)
	}
}
