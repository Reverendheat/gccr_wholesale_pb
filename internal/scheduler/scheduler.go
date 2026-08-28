// Package scheduler runs the recurring-order cron job.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/reverendheat/gccr_invoice/internal/delivery"
	"github.com/reverendheat/gccr_invoice/internal/orders"
	"github.com/reverendheat/gccr_invoice/internal/orders/fsm"
	"github.com/reverendheat/gccr_invoice/internal/square"
)

// Register wires a cron job that fires every 30 minutes and creates any
// scheduled orders that are due.
func Register(app core.App, sq *square.Client, deliveryQuoter delivery.Quoter) {
	app.Cron().MustAdd("scheduledOrders", "*/30 * * * *", func() {
		if err := process(app, sq, deliveryQuoter); err != nil {
			log.Printf("scheduled orders cron: %v", err)
		}
	})

	app.Cron().MustAdd("reconcile_invoices", "*/15 * * * *", func() {
		if err := reconcileInvoices(app, sq); err != nil {
			log.Printf("reconcile invoices cron: %v", err)
		}
	})
}

// process finds all active scheduled orders whose next_run_at is in the past
// (or now), creates an order for each, and advances next_run_at.
func process(app core.App, sq *square.Client, deliveryQuoter delivery.Quoter) error {
	due, err := app.FindRecordsByFilter(
		"scheduledOrders",
		"active = true && next_run_at <= @now",
		"+next_run_at", 500, 0,
	)
	if err != nil {
		return fmt.Errorf("fetch due scheduled orders: %w", err)
	}

	for _, sr := range due {
		if err := processOne(app, sq, deliveryQuoter, sr); err != nil {
			log.Printf("scheduled order %s: %v", sr.Id, err)
		}
	}
	return nil
}

// processOne creates a single order for one scheduledOrders record and
// advances its next_run_at timestamp.
func processOne(app core.App, sq *square.Client, deliveryQuoter delivery.Quoter, sr *core.Record) error {
	ctx := context.Background()
	var err error

	var lineItemsRaw []map[string]any
	if err := sr.UnmarshalJSONField("lineItems", &lineItemsRaw); err != nil {
		return fmt.Errorf("unmarshal lineItems: %w", err)
	}

	items := make([]orders.LineItem, 0, len(lineItemsRaw))
	for _, li := range lineItemsRaw {
		qty := 1
		if q, ok := li["quantity"].(float64); ok {
			qty = int(q)
		}
		note, _ := li["note"].(string)
		grindModifierID, _ := li["grind_modifier_id"].(string)
		items = append(items, orders.LineItem{
			VariationID:     fmt.Sprint(li["variation_id"]),
			GrindModifierID: grindModifierID,
			Quantity:        qty,
			Note:            note,
		})
	}

	fulfillment := orders.Fulfillment{}
	if sr.GetString("fulfillment") != "" {
		if err := sr.UnmarshalJSONField("fulfillment", &fulfillment); err != nil {
			return fmt.Errorf("unmarshal fulfillment: %w", err)
		}
	}
	fulfillment, err = orders.NormalizeFulfillment(fulfillment)
	if err != nil {
		return fmt.Errorf("validate fulfillment: %w", err)
	}

	customer, err := app.FindRecordById("customers", sr.GetString("customer"))
	if err != nil {
		return fmt.Errorf("find schedule customer: %w", err)
	}
	squareCustomerID := customer.GetString("squareCustomerId")
	if squareCustomerID == "" {
		return fmt.Errorf("schedule customer is not linked to Square")
	}

	items, err = orders.LockPrices(ctx, sq, squareCustomerID, items)
	if err != nil {
		return fmt.Errorf("lock order pricing: %w", err)
	}
	fulfillment, _, err = orders.QuoteFulfillment(ctx, deliveryQuoter, fulfillment, items)
	if err != nil {
		return fmt.Errorf("quote fulfillment: %w", err)
	}

	actorName := customer.GetString("name")
	if actorName == "" {
		actorName = customer.Email()
	}
	pbOrder, err := orders.CreateWithPlacement(
		app, customer.Id, sr.GetString("company"),
		items, fulfillment, sr.GetString("notes"),
		orders.Placement{Actor: orders.Actor{
			Type: orders.ActorCustomer, ID: customer.Id, Name: actorName,
		}},
	)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}

	log.Printf("scheduled order %s: created order %s", sr.Id, pbOrder.Id)

	// Advance next_run_at by the configured frequency.
	nextRun := advanceBy(time.Now().UTC(), sr.GetString("frequency"))
	sr.Set("next_run_at", nextRun.Format("2006-01-02 15:04:05.000Z"))
	if err := app.Save(sr); err != nil {
		return fmt.Errorf("advance next_run_at: %w", err)
	}

	return nil
}

// advanceBy computes the next run timestamp for a given frequency.
func advanceBy(from time.Time, frequency string) time.Time {
	switch frequency {
	case "weekly":
		return from.AddDate(0, 0, 7)
	case "biweekly":
		return from.AddDate(0, 0, 14)
	case "monthly":
		return from.AddDate(0, 1, 0)
	case "quarterly":
		return from.AddDate(0, 3, 0)
	default:
		return from.AddDate(0, 0, 7)
	}
}

// reconcileInvoices polls Square for invoice status and public URLs on orders
// with Square invoices. This catches payments missed by webhooks and backfills
// invoice URLs for records created before the URL field existed.
func reconcileInvoices(app core.App, sq *square.Client) error {
	invoiced, err := app.FindRecordsByFilter(
		"orders",
		"squareInvoiceId != '' && (status = 'invoiced' || squareInvoiceUrl = '')",
		"+created", 200, 0,
	)
	if err != nil {
		return fmt.Errorf("fetch invoiced orders: %w", err)
	}

	ctx := context.Background()
	for _, order := range invoiced {
		invoiceID := order.GetString("squareInvoiceId")
		inv, err := sq.GetInvoice(ctx, invoiceID)
		if err != nil {
			log.Printf("reconcile: order %s: could not fetch invoice %s: %v", order.Id, invoiceID, err)
			continue
		}

		changed := false
		if order.GetString("squareInvoiceUrl") == "" && inv.PublicURL != nil {
			order.Set("squareInvoiceUrl", *inv.PublicURL)
			changed = true
		}

		if inv.Status != nil {
			if event, ok := fsm.EventForSquareInvoiceStatus(string(*inv.Status)); ok {
				next, err := fsm.Apply(fsm.Status(order.GetString("status")), event)
				if err != nil {
					log.Printf("reconcile: order %s: could not apply %s: %v", order.Id, event, err)
				} else if string(next) != order.GetString("status") {
					order.Set("status", string(next))
					changed = true
				}
			}
		}

		if changed {
			if err := app.Save(order); err != nil {
				log.Printf("reconcile: order %s: could not update invoice state: %v", order.Id, err)
			} else {
				log.Printf("reconcile: order %s updated from invoice %s", order.Id, invoiceID)
			}
		}
	}

	return nil
}
