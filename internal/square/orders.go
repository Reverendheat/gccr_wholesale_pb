package square

import (
	"context"
	"fmt"

	squaresdk "github.com/square/square-go-sdk/v3"
)

// CreateOrder pushes a new order to Square and returns the created Order.
func (c *Client) CreateOrder(
	ctx context.Context,
	locationID string,
	squareCustomerID string,
	lineItems []*squaresdk.OrderLineItem,
	referenceID string,
	idempotencyKey string,
) (*squaresdk.Order, error) {
	if locationID == "" {
		return nil, fmt.Errorf("square: locationID is required")
	}
	if squareCustomerID == "" {
		return nil, fmt.Errorf("square: squareCustomerID is required")
	}
	if len(lineItems) == 0 {
		return nil, fmt.Errorf("square: at least one line item is required")
	}

	resp, err := c.SDK.Orders.Create(ctx, &squaresdk.CreateOrderRequest{
		IdempotencyKey: squaresdk.String(idempotencyKey),
		Order: &squaresdk.Order{
			LocationID:  locationID,
			CustomerID:  squaresdk.String(squareCustomerID),
			LineItems:   lineItems,
			ReferenceID: squaresdk.String(referenceID),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("square: CreateOrder: %w", err)
	}

	return resp.Order, nil
}
