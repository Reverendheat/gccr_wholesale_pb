package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouteServiceGeocodesAndRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/geocode/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "test-key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("text") == "" || r.URL.Query().Get("boundary.country") != "US" {
			t.Fatalf("unexpected geocode query: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"features": []any{map[string]any{
				"geometry": map[string]any{"coordinates": []float64{-77.4, 37.5}},
			}},
		})
	})
	mux.HandleFunc("/v2/directions/driving-car", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "test-key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var body struct {
			Coordinates [][]float64 `json:"coordinates"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Coordinates) != 2 || body.Coordinates[0][0] != -77.5 || body.Coordinates[1][1] != 37.5 {
			t.Fatalf("coordinates = %#v", body.Coordinates)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"routes": []any{map[string]any{"summary": map[string]any{"distance": 12345.0}}},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewOpenRouteService("test-key", srv.URL, srv.Client())

	coords, err := client.Geocode(context.Background(), Address{
		Line1: "123 Main St", City: "Richmond", State: "VA", PostalCode: "23220", Country: "US",
	})
	if err != nil {
		t.Fatal(err)
	}
	if coords.Longitude != -77.4 || coords.Latitude != 37.5 {
		t.Fatalf("coordinates = %+v", coords)
	}

	distance, err := client.DrivingDistanceMeters(context.Background(), Coordinates{Longitude: -77.5, Latitude: 37.6}, coords)
	if err != nil {
		t.Fatal(err)
	}
	if distance != 12345 {
		t.Fatalf("distance = %v", distance)
	}
}

func TestOpenRouteServiceRejectsUnresolvedAddress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"features": []any{}})
	}))
	defer srv.Close()

	client := NewOpenRouteService("test-key", srv.URL, srv.Client())
	if _, err := client.Geocode(context.Background(), Address{Line1: "Unknown"}); err == nil {
		t.Fatal("expected unresolved address error")
	}
}
