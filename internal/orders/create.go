// Package orders provides shared order creation logic used by both HTTP route
// handlers and the scheduled-order cron runner.
package orders

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
	"github.com/reverendheat/gccr_invoice/internal/square"
	squaresdk "github.com/square/square-go-sdk/v3"
)

// LineItem represents a single item in an order.
type LineItem struct {
	VariationID string
	Quantity    int
	Note        string
}

// Create saves a new order record in PocketBase, submits it to Square, and
// writes the resulting Square order ID back to the PocketBase record.
// idempotencyKey must be globally unique per attempt (use the PocketBase record
// ID or a derived value) to safely retry without double-charging.
func Create(
	ctx context.Context,
	app core.App,
	sq *square.Client,
	locationID, customerID, companyID, squareCustomerID string,
	items []LineItem,
	notes, idempotencyKey string,
) (*core.Record, error) {
	lineItemsSnapshot := make([]map[string]any, len(items))
	squareLineItems := make([]*squaresdk.OrderLineItem, len(items))
	for i, li := range items {
		lineItemsSnapshot[i] = map[string]any{
			"variation_id": li.VariationID,
			"quantity":     li.Quantity,
			"note":         li.Note,
		}
		squareLineItems[i] = &squaresdk.OrderLineItem{
			CatalogObjectID: squaresdk.String(li.VariationID),
			Quantity:        strconv.Itoa(li.Quantity),
			Note:            squaresdk.String(li.Note),
		}
	}

	ordersCollection, err := app.FindCollectionByNameOrId("orders")
	if err != nil {
		return nil, fmt.Errorf("orders collection not found: %w", err)
	}

	pbOrder := core.NewRecord(ordersCollection)
	pbOrder.Set("customer", customerID)
	pbOrder.Set("company", companyID)
	pbOrder.Set("status", "pending")
	pbOrder.Set("notes", notes)
	pbOrder.Set("lineItems", lineItemsSnapshot)

	if err := app.Save(pbOrder); err != nil {
		return nil, fmt.Errorf("save order: %w", err)
	}

	squareOrder, err := sq.CreateOrder(
		ctx,
		locationID,
		squareCustomerID,
		squareLineItems,
		pbOrder.Id,
		idempotencyKey,
	)
	if err != nil {
		_ = app.Delete(pbOrder)
		return nil, fmt.Errorf("create square order: %w", err)
	}

	pbOrder.Set("squareOrderId", *squareOrder.ID)
	if err := app.Save(pbOrder); err != nil {
		return nil, fmt.Errorf("update order with square id: %w", err)
	}

	// Re-fetch so the response includes server-populated timestamps (created, updated).
	refreshed, err := app.FindRecordById("orders", pbOrder.Id)
	if err != nil {
		return nil, fmt.Errorf("refresh order: %w", err)
	}
	return refreshed, nil
}
