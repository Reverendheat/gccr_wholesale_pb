// Package scheduler runs the recurring-order cron job.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/reverendheat/gccr_invoice/internal/orders"
	"github.com/reverendheat/gccr_invoice/internal/square"
)

// Register wires a cron job that fires every 30 minutes and creates any
// scheduled orders that are due.
func Register(app core.App, sq *square.Client, locationID string) {
	app.Cron().MustAdd("scheduled_orders", "*/30 * * * *", func() {
		if err := process(app, sq, locationID); err != nil {
			log.Printf("scheduled orders cron: %v", err)
		}
	})
}

// process finds all active scheduled orders whose next_run_at is in the past
// (or now), creates an order for each, and advances next_run_at.
func process(app core.App, sq *square.Client, locationID string) error {
	due, err := app.FindRecordsByFilter(
		"scheduled_orders",
		"active = true && next_run_at <= @now",
		"+next_run_at", 500, 0,
	)
	if err != nil {
		return fmt.Errorf("fetch due scheduled orders: %w", err)
	}

	for _, sr := range due {
		if err := processOne(app, sq, locationID, sr); err != nil {
			log.Printf("scheduled order %s: %v", sr.Id, err)
		}
	}
	return nil
}

// processOne creates a single order for one scheduled_orders record and
// advances its next_run_at timestamp.
func processOne(app core.App, sq *square.Client, locationID string, sr *core.Record) error {
	ctx := context.Background()

	customerRecord, err := app.FindRecordById("customers", sr.GetString("customer"))
	if err != nil {
		return fmt.Errorf("find customer: %w", err)
	}

	squareCustomerID := customerRecord.GetString("square_customer_id")
	if squareCustomerID == "" {
		return fmt.Errorf("customer %s has no square_customer_id", customerRecord.Id)
	}

	var lineItemsRaw []map[string]any
	if err := sr.UnmarshalJSONField("line_items", &lineItemsRaw); err != nil {
		return fmt.Errorf("unmarshal line_items: %w", err)
	}

	items := make([]orders.LineItem, 0, len(lineItemsRaw))
	for _, li := range lineItemsRaw {
		qty := 1
		if q, ok := li["quantity"].(float64); ok {
			qty = int(q)
		}
		items = append(items, orders.LineItem{
			VariationID: fmt.Sprint(li["variation_id"]),
			Quantity:    qty,
			Note:        fmt.Sprint(li["note"]),
		})
	}

	// idempotency key embeds both the scheduled order ID and the current date
	// so re-runs on the same day are safe.
	idempotencyKey := fmt.Sprintf("sched-%s-%s", sr.Id, time.Now().UTC().Format("20060102"))

	pbOrder, err := orders.Create(
		ctx, app, sq,
		locationID, sr.GetString("customer"), squareCustomerID,
		items, sr.GetString("notes"), idempotencyKey,
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
