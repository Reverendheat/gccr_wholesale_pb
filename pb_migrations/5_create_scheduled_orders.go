package pb_migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		customersCol, err := app.FindCollectionByNameOrId("customers")
		if err != nil {
			return fmt.Errorf("could not find customers collection: %w", err)
		}

		collection := core.NewBaseCollection("scheduled_orders")

		collection.ListRule = types.Pointer(
			`customer.id = @request.auth.id || @request.auth.collectionName = "users"`,
		)
		collection.ViewRule = types.Pointer(
			`customer.id = @request.auth.id || @request.auth.collectionName = "users"`,
		)
		collection.CreateRule = nil
		collection.UpdateRule = types.Pointer(`@request.auth.collectionName = "users"`)
		collection.DeleteRule = nil

		collection.Fields.Add(
			&core.RelationField{
				Name:         "customer",
				CollectionId: customersCol.Id,
				Required:     true,
				MaxSelect:    1,
			},
			&core.SelectField{
				Name:     "frequency",
				Required: true,
				Values:   []string{"weekly", "biweekly", "monthly", "quarterly"},
			},
			// JSON snapshot of line items: [{variation_id, quantity, note}]
			&core.JSONField{
				Name:     "line_items",
				Required: true,
			},
			&core.TextField{
				Name:     "notes",
				Required: false,
				Max:      2000,
			},
			// Timestamp of the next scheduled order creation
			&core.DateField{
				Name:     "next_run_at",
				Required: true,
			},
			&core.BoolField{
				Name:     "active",
				Required: false,
			},
		)

		collection.AddIndex("idx_scheduled_orders_customer", false, "customer", "")
		collection.AddIndex("idx_scheduled_orders_next_run", false, "next_run_at", "")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("scheduled_orders")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
