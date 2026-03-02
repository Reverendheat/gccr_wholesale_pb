package square

import (
	"context"
	"fmt"
)

// OrderLineItem is a single line item for a Square order.
type OrderLineItem struct {
	// CatalogObjectID is the Square item variation ID.
	CatalogObjectID string `json:"catalog_object_id"`
	// Quantity must be a string per the Square API spec.
	Quantity string `json:"quantity"`
	// Note is an optional customer-facing note for this line item.
	Note string `json:"note,omitempty"`
}

// Order is a minimal representation of a Square Order.
type Order struct {
	ID         string          `json:"id"`
	LocationID string          `json:"location_id"`
	CustomerID string          `json:"customer_id"`
	LineItems  []OrderLineItem `json:"line_items"`
	State      string          `json:"state"`
}

type createOrderRequest struct {
	Order        createOrderBody `json:"order"`
	IdempotencyKey string        `json:"idempotency_key"`
}

type createOrderBody struct {
	LocationID string          `json:"location_id"`
	CustomerID string          `json:"customer_id"`
	LineItems  []OrderLineItem `json:"line_items"`
	// ReferenceID ties the Square order back to our PocketBase order ID.
	ReferenceID string `json:"reference_id,omitempty"`
}

type createOrderResponse struct {
	Order Order `json:"order"`
}

// CreateOrder pushes a new order to Square and returns the created Order.
// idempotencyKey should be a unique value per attempt (e.g. PocketBase order ID).
func (c *Client) CreateOrder(
	ctx context.Context,
	locationID string,
	squareCustomerID string,
	lineItems []OrderLineItem,
	referenceID string,
	idempotencyKey string,
) (*Order, error) {
	if locationID == "" {
		return nil, fmt.Errorf("square: locationID is required")
	}
	if squareCustomerID == "" {
		return nil, fmt.Errorf("square: squareCustomerID is required")
	}
	if len(lineItems) == 0 {
		return nil, fmt.Errorf("square: at least one line item is required")
	}

	reqBody := createOrderRequest{
		IdempotencyKey: idempotencyKey,
		Order: createOrderBody{
			LocationID:  locationID,
			CustomerID:  squareCustomerID,
			LineItems:   lineItems,
			ReferenceID: referenceID,
		},
	}

	var resp createOrderResponse
	if err := c.doPost(ctx, "/v2/orders", reqBody, &resp); err != nil {
		return nil, fmt.Errorf("square: CreateOrder: %w", err)
	}

	return &resp.Order, nil
}
