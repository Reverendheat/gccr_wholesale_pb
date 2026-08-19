package square

import (
	"context"
	"fmt"

	"github.com/reverendheat/gccr_invoice/internal/delivery"
	squaresdk "github.com/square/square-go-sdk/v3"
)

// DeliveryOrigin returns coordinates or a geocodable address for a Square location.
func (c *Client) DeliveryOrigin(ctx context.Context, locationID string) (delivery.Origin, error) {
	if locationID == "" {
		return delivery.Origin{}, fmt.Errorf("square: locationID is required")
	}
	response, err := c.SDK.Locations.Get(ctx, &squaresdk.GetLocationsRequest{LocationID: locationID})
	if err != nil {
		return delivery.Origin{}, fmt.Errorf("square: get location %q: %w", locationID, err)
	}
	if response.Location == nil {
		return delivery.Origin{}, fmt.Errorf("square: location %q was not returned", locationID)
	}

	origin := delivery.Origin{}
	if coordinates := response.Location.Coordinates; coordinates != nil && coordinates.Longitude != nil && coordinates.Latitude != nil {
		origin.Coordinates = &delivery.Coordinates{Longitude: *coordinates.Longitude, Latitude: *coordinates.Latitude}
	}
	if address := response.Location.Address; address != nil {
		origin.Address = delivery.Address{
			Line1:      stringValue(address.AddressLine1),
			Line2:      stringValue(address.AddressLine2),
			City:       stringValue(address.Locality),
			State:      stringValue(address.AdministrativeDistrictLevel1),
			PostalCode: stringValue(address.PostalCode),
		}
		if address.Country != nil {
			origin.Address.Country = string(*address.Country)
		}
	}
	if origin.Coordinates == nil && origin.Address.Line1 == "" {
		return delivery.Origin{}, fmt.Errorf("square: location %q has no coordinates or physical address", locationID)
	}
	return origin, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
