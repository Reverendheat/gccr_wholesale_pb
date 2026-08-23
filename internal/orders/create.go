// Package orders provides shared order creation logic used by both HTTP route
// handlers and the scheduled-order cron runner.
package orders

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/reverendheat/gccr_invoice/internal/square"
	squaresdk "github.com/square/square-go-sdk/v3"
)

// LineItem is an immutable order-time snapshot. Prices are locked when the
// customer submits the order, before any Square order or invoice is created.
type LineItem struct {
	VariationID    string `json:"variation_id"`
	Name           string `json:"name"`
	Quantity       int    `json:"quantity"`
	Note           string `json:"note"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Currency       string `json:"currency"`
}

const (
	ActorCustomer = "customer"
	ActorStaff    = "staff"
)

type Actor struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Placement struct {
	Actor Actor
	Note  string
}

type EditAudit struct {
	Actor    Actor  `json:"actor"`
	Reason   string `json:"reason"`
	EditedAt string `json:"edited_at"`
}

// MerchandiseSubtotal returns locked merchandise total before fulfillment fees.
func MerchandiseSubtotal(items []LineItem) (int64, string, error) {
	if len(items) == 0 {
		return 0, "", fmt.Errorf("at least one line item is required")
	}
	currency := items[0].Currency
	var subtotal int64
	for _, item := range items {
		if item.Quantity <= 0 || item.UnitPriceCents < 0 || item.Currency == "" {
			return 0, "", fmt.Errorf("line items require positive quantity, non-negative price, and currency")
		}
		if item.Currency != currency {
			return 0, "", fmt.Errorf("all line items must use the same currency")
		}
		subtotal += item.UnitPriceCents * int64(item.Quantity)
	}
	return subtotal, currency, nil
}

// LockPrices resolves requested variations against the target customer's current
// wholesale catalog and returns submission-time price snapshots.
func LockPrices(ctx context.Context, sq *square.Client, squareCustomerID string, requested []LineItem) ([]LineItem, error) {
	catalog, err := sq.GetWholesaleCatalog(ctx, squareCustomerID)
	if err != nil {
		return nil, err
	}

	available := make(map[string]LineItem)
	for _, item := range catalog {
		if item.ItemData == nil {
			continue
		}
		itemName := ""
		if item.ItemData.Name != nil {
			itemName = *item.ItemData.Name
		}
		for _, variation := range item.ItemData.Variations {
			if variation == nil || variation.ItemVariation == nil || variation.ItemVariation.ID == "" || variation.ItemVariation.ItemVariationData == nil {
				continue
			}
			data := variation.ItemVariation.ItemVariationData
			price := data.PriceMoney
			pricingType := data.PricingType
			if pricingType != nil && *pricingType == squaresdk.CatalogPricingTypeVariablePricing {
				continue
			}
			if price == nil || price.Amount == nil || price.Currency == nil {
				continue
			}

			name := itemName
			if data.Name != nil && *data.Name != "" && !strings.EqualFold(*data.Name, "Default") {
				name = strings.TrimSpace(itemName + " - " + *data.Name)
			}
			available[variation.ItemVariation.ID] = LineItem{
				VariationID:    variation.ItemVariation.ID,
				Name:           name,
				UnitPriceCents: *price.Amount,
				Currency:       string(*price.Currency),
			}
		}
	}

	locked := make([]LineItem, len(requested))
	for i, item := range requested {
		snapshot, ok := available[item.VariationID]
		if !ok {
			return nil, fmt.Errorf("variation %q is unavailable in the wholesale catalog", item.VariationID)
		}
		snapshot.Quantity = item.Quantity
		snapshot.Note = item.Note
		locked[i] = snapshot
	}
	return locked, nil
}

// Create saves a legacy local order without an explicit placement actor.
func Create(
	app core.App,
	customerID, companyID string,
	items []LineItem,
	fulfillment Fulfillment,
	notes string,
) (*core.Record, error) {
	return create(app, customerID, companyID, items, fulfillment, notes, Placement{})
}

// CreateWithPlacement saves an order with immutable actor audit details.
func CreateWithPlacement(
	app core.App,
	customerID, companyID string,
	items []LineItem,
	fulfillment Fulfillment,
	notes string,
	placement Placement,
) (*core.Record, error) {
	if placement.Actor.Type != ActorCustomer && placement.Actor.Type != ActorStaff {
		return nil, fmt.Errorf("placement actor type must be customer or staff")
	}
	if placement.Actor.ID == "" || strings.TrimSpace(placement.Actor.Name) == "" {
		return nil, fmt.Errorf("placement actor requires id and name")
	}
	placement.Note = strings.TrimSpace(placement.Note)
	return create(app, customerID, companyID, items, fulfillment, notes, placement)
}

func create(
	app core.App,
	customerID, companyID string,
	items []LineItem,
	fulfillment Fulfillment,
	notes string,
	placement Placement,
) (*core.Record, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one line item is required")
	}
	for _, item := range items {
		if item.VariationID == "" || item.Quantity <= 0 || item.Currency == "" {
			return nil, fmt.Errorf("line items require variation, positive quantity, and locked currency")
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
	pbOrder.Set("fulfillment", fulfillment)
	pbOrder.Set("notes", notes)
	pbOrder.Set("lineItems", items)
	if placement.Actor.Type != "" {
		pbOrder.Set("placedBy", placement.Actor)
		pbOrder.Set("placementNote", placement.Note)
	}

	if err := app.Save(pbOrder); err != nil {
		return nil, fmt.Errorf("save order: %w", err)
	}

	refreshed, err := app.FindRecordById("orders", pbOrder.Id)
	if err != nil {
		return nil, fmt.Errorf("refresh order: %w", err)
	}
	return refreshed, nil
}

// UpdatePending replaces the editable snapshot of a pending local order.
// Prices must already be locked against the current wholesale catalog.
func UpdatePending(
	app core.App,
	order *core.Record,
	items []LineItem,
	fulfillment Fulfillment,
	notes string,
) (*core.Record, error) {
	if order.GetString("status") != "pending" ||
		order.GetString("squareOrderId") != "" ||
		order.GetString("squareInvoiceId") != "" {
		return nil, fmt.Errorf("only pending orders not yet submitted to Square can be edited")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one line item is required")
	}
	for _, item := range items {
		if item.VariationID == "" || item.Quantity <= 0 || item.Currency == "" {
			return nil, fmt.Errorf("line items require variation, positive quantity, and locked currency")
		}
	}

	order.Set("lineItems", items)
	order.Set("fulfillment", fulfillment)
	order.Set("notes", notes)
	if err := app.Save(order); err != nil {
		return nil, fmt.Errorf("save pending order: %w", err)
	}

	updated, err := app.FindRecordById("orders", order.Id)
	if err != nil {
		return nil, fmt.Errorf("refresh pending order: %w", err)
	}
	return updated, nil
}

// UpdateByStaff replaces a pending or confirmed order before Square submission.
func UpdateByStaff(
	app core.App,
	order *core.Record,
	items []LineItem,
	fulfillment Fulfillment,
	notes string,
	audit EditAudit,
) (*core.Record, error) {
	status := order.GetString("status")
	if (status != "pending" && status != "confirmed") ||
		order.GetString("squareOrderId") != "" ||
		order.GetString("squareInvoiceId") != "" {
		return nil, fmt.Errorf("staff can only edit pending or confirmed orders before Square submission")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one line item is required")
	}
	for _, item := range items {
		if item.VariationID == "" || item.Quantity <= 0 || item.Currency == "" {
			return nil, fmt.Errorf("line items require variation, positive quantity, and locked currency")
		}
	}
	if audit.Actor.Type != ActorStaff || audit.Actor.ID == "" || strings.TrimSpace(audit.Actor.Name) == "" {
		return nil, fmt.Errorf("staff edit requires staff actor")
	}
	audit.Reason = strings.TrimSpace(audit.Reason)
	if audit.Reason == "" {
		return nil, fmt.Errorf("staff edit reason is required")
	}
	if audit.EditedAt == "" {
		audit.EditedAt = time.Now().UTC().Format(time.RFC3339)
	}

	var history []EditAudit
	if order.GetString("editHistory") != "" {
		if err := order.UnmarshalJSONField("editHistory", &history); err != nil {
			return nil, fmt.Errorf("decode order edit history: %w", err)
		}
	}
	history = append(history, audit)
	order.Set("lineItems", items)
	order.Set("fulfillment", fulfillment)
	order.Set("notes", notes)
	order.Set("editHistory", history)
	if err := app.Save(order); err != nil {
		return nil, fmt.Errorf("save staff order edit: %w", err)
	}
	updated, err := app.FindRecordById("orders", order.Id)
	if err != nil {
		return nil, fmt.Errorf("refresh staff-edited order: %w", err)
	}
	return updated, nil
}

// SubmitToSquare creates the delayed Square order from its locked local
// snapshot and persists the resulting Square ID. Repeated calls are safe.
func SubmitToSquare(
	ctx context.Context,
	app core.App,
	sq *square.Client,
	locationID, squareCustomerID string,
	order *core.Record,
) (string, error) {
	if existing := order.GetString("squareOrderId"); existing != "" {
		return existing, nil
	}

	var items []LineItem
	if err := order.UnmarshalJSONField("lineItems", &items); err != nil {
		return "", fmt.Errorf("decode locked line items: %w", err)
	}
	squareItems, err := toSquareLineItems(items)
	if err != nil {
		return "", err
	}
	var fulfillment Fulfillment
	if err := order.UnmarshalJSONField("fulfillment", &fulfillment); err != nil {
		return "", fmt.Errorf("decode fulfillment snapshot: %w", err)
	}
	if fulfillment.Method == FulfillmentDelivery && fulfillment.FeeCents > 0 {
		currency, err := squaresdk.NewCurrencyFromString(fulfillment.Currency)
		if err != nil {
			return "", fmt.Errorf("delivery fee currency: %w", err)
		}
		squareItems = append(squareItems, &squaresdk.OrderLineItem{
			Name:     squaresdk.String("Local delivery"),
			Quantity: "1",
			BasePriceMoney: &squaresdk.Money{
				Amount:   squaresdk.Int64(fulfillment.FeeCents),
				Currency: &currency,
			},
		})
	}

	squareOrder, err := sq.CreateOrder(
		ctx, locationID, squareCustomerID, squareItems,
		order.Id, "create-order-"+order.Id,
	)
	if err != nil {
		return "", err
	}
	if squareOrder.ID == nil || *squareOrder.ID == "" {
		return "", fmt.Errorf("Square created order without an ID")
	}

	order.Set("squareOrderId", *squareOrder.ID)
	if err := app.Save(order); err != nil {
		return "", fmt.Errorf("persist Square order ID: %w", err)
	}
	return *squareOrder.ID, nil
}

func toSquareLineItems(items []LineItem) ([]*squaresdk.OrderLineItem, error) {
	out := make([]*squaresdk.OrderLineItem, len(items))
	for i, item := range items {
		currency, err := squaresdk.NewCurrencyFromString(item.Currency)
		if err != nil {
			return nil, fmt.Errorf("line item %q currency: %w", item.VariationID, err)
		}
		out[i] = &squaresdk.OrderLineItem{
			CatalogObjectID: squaresdk.String(item.VariationID),
			Quantity:        strconv.Itoa(item.Quantity),
			Note:            squaresdk.String(item.Note),
			BasePriceMoney: &squaresdk.Money{
				Amount:   squaresdk.Int64(item.UnitPriceCents),
				Currency: &currency,
			},
		}
	}
	return out, nil
}
