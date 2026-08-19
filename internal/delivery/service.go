// Package delivery calculates authoritative local-delivery quotes.
package delivery

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
)

const (
	metersPerMile = 1609.344
	PricingRuleV1 = "local-delivery-v1"
)

var (
	ErrOutsideDeliveryArea = errors.New("delivery address is more than 30 driving miles from our location")
	ErrAddressNotFound     = errors.New("delivery address could not be located")
)

type Address struct {
	Line1      string `json:"address_line_1"`
	Line2      string `json:"address_line_2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

func (a Address) String() string {
	parts := []string{a.Line1, a.Line2, a.City, a.State, a.PostalCode, a.Country}
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			clean = append(clean, part)
		}
	}
	return strings.Join(clean, ", ")
}

type Coordinates struct {
	Longitude float64
	Latitude  float64
}

type Origin struct {
	Address     Address
	Coordinates *Coordinates
}

type Quote struct {
	DistanceMeters float64 `json:"distance_meters"`
	DistanceMiles  float64 `json:"distance_miles"`
	FeeCents       int64   `json:"fee_cents"`
	Currency       string  `json:"currency"`
	PricingRule    string  `json:"pricing_rule"`
}

type Policy struct {
	MaxMiles         float64
	FreeMinimumCents int64
	RateCentsPerMile int64
}

type Options struct {
	MaxMiles         float64 `json:"max_miles"`
	FreeMinimumCents int64   `json:"free_minimum_cents"`
	RateCentsPerMile int64   `json:"rate_cents_per_mile"`
}

func (p Policy) Options() Options {
	return Options{
		MaxMiles: p.MaxMiles, FreeMinimumCents: p.FreeMinimumCents, RateCentsPerMile: p.RateCentsPerMile,
	}
}

func (p Policy) Calculate(distanceMeters float64, merchandiseSubtotalCents int64, currency string) (Quote, error) {
	if distanceMeters < 0 {
		return Quote{}, fmt.Errorf("delivery distance cannot be negative")
	}
	miles := distanceMeters / metersPerMile
	if miles > p.MaxMiles {
		return Quote{}, ErrOutsideDeliveryArea
	}
	fee := int64(0)
	if merchandiseSubtotalCents < p.FreeMinimumCents {
		fee = int64(math.Ceil(miles)) * p.RateCentsPerMile
	}
	return Quote{
		DistanceMeters: distanceMeters,
		DistanceMiles:  miles,
		FeeCents:       fee,
		Currency:       currency,
		PricingRule:    PricingRuleV1,
	}, nil
}

type OriginProvider interface {
	DeliveryOrigin(ctx context.Context, locationID string) (Origin, error)
}

type Router interface {
	Geocode(ctx context.Context, address Address) (Coordinates, error)
	DrivingDistanceMeters(ctx context.Context, origin, destination Coordinates) (float64, error)
}

type Quoter interface {
	Quote(ctx context.Context, destination Address, merchandiseSubtotalCents int64, currency string) (Quote, error)
}

type Calculator interface {
	Quoter
	Options() Options
}

type Service struct {
	locationID string
	origins    OriginProvider
	router     Router
	policy     Policy

	mu           sync.RWMutex
	cachedOrigin *Coordinates
}

func NewService(locationID string, origins OriginProvider, router Router, policy Policy) *Service {
	return &Service{locationID: locationID, origins: origins, router: router, policy: policy}
}

func (s *Service) Options() Options {
	return s.policy.Options()
}

func (s *Service) Quote(ctx context.Context, destination Address, merchandiseSubtotalCents int64, currency string) (Quote, error) {
	origin, err := s.originCoordinates(ctx)
	if err != nil {
		return Quote{}, fmt.Errorf("resolve delivery origin: %w", err)
	}
	destinationCoordinates, err := s.router.Geocode(ctx, destination)
	if err != nil {
		return Quote{}, fmt.Errorf("resolve delivery address: %w", err)
	}
	distance, err := s.router.DrivingDistanceMeters(ctx, origin, destinationCoordinates)
	if err != nil {
		return Quote{}, fmt.Errorf("calculate driving distance: %w", err)
	}
	return s.policy.Calculate(distance, merchandiseSubtotalCents, currency)
}

func (s *Service) originCoordinates(ctx context.Context) (Coordinates, error) {
	s.mu.RLock()
	if s.cachedOrigin != nil {
		origin := *s.cachedOrigin
		s.mu.RUnlock()
		return origin, nil
	}
	s.mu.RUnlock()

	origin, err := s.origins.DeliveryOrigin(ctx, s.locationID)
	if err != nil {
		return Coordinates{}, err
	}
	coordinates := origin.Coordinates
	if coordinates == nil {
		geocoded, err := s.router.Geocode(ctx, origin.Address)
		if err != nil {
			return Coordinates{}, fmt.Errorf("geocode Square location address: %w", err)
		}
		coordinates = &geocoded
	}

	s.mu.Lock()
	s.cachedOrigin = coordinates
	s.mu.Unlock()
	return *coordinates, nil
}
