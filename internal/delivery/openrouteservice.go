package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultOpenRouteServiceURL = "https://api.openrouteservice.org"

type OpenRouteService struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewOpenRouteService(apiKey, baseURL string, httpClient *http.Client) *OpenRouteService {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOpenRouteServiceURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 12 * time.Second}
	}
	return &OpenRouteService{
		apiKey: strings.TrimSpace(apiKey), baseURL: strings.TrimRight(baseURL, "/"), http: httpClient,
	}
}

func (c *OpenRouteService) Geocode(ctx context.Context, address Address) (Coordinates, error) {
	endpoint, err := url.Parse(c.baseURL + "/geocode/search")
	if err != nil {
		return Coordinates{}, err
	}
	query := endpoint.Query()
	query.Set("text", address.String())
	query.Set("boundary.country", "US")
	query.Set("size", "1")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Coordinates{}, err
	}
	request.Header.Set("Authorization", c.apiKey)

	var response struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := c.doJSON(request, &response); err != nil {
		return Coordinates{}, fmt.Errorf("openrouteservice geocode: %w", err)
	}
	if len(response.Features) == 0 || len(response.Features[0].Geometry.Coordinates) < 2 {
		return Coordinates{}, ErrAddressNotFound
	}
	return Coordinates{
		Longitude: response.Features[0].Geometry.Coordinates[0],
		Latitude:  response.Features[0].Geometry.Coordinates[1],
	}, nil
}

func (c *OpenRouteService) DrivingDistanceMeters(ctx context.Context, origin, destination Coordinates) (float64, error) {
	body, err := json.Marshal(map[string]any{
		"coordinates": [][]float64{
			{origin.Longitude, origin.Latitude},
			{destination.Longitude, destination.Latitude},
		},
		"instructions": false,
	})
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/directions/driving-car", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Authorization", c.apiKey)
	request.Header.Set("Content-Type", "application/json")

	var response struct {
		Routes []struct {
			Summary struct {
				Distance float64 `json:"distance"`
			} `json:"summary"`
		} `json:"routes"`
	}
	if err := c.doJSON(request, &response); err != nil {
		return 0, fmt.Errorf("openrouteservice directions: %w", err)
	}
	if len(response.Routes) == 0 || response.Routes[0].Summary.Distance <= 0 {
		return 0, fmt.Errorf("openrouteservice returned no driving route")
	}
	return response.Routes[0].Summary.Distance, nil
}

func (c *OpenRouteService) doJSON(request *http.Request, target any) error {
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
