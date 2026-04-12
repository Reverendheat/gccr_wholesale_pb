package pb_migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Resolve the customers collection to get its real system ID
		customersCol, err := app.FindCollectionByNameOrId("customers")
		if err != nil {
			return fmt.Errorf("could not find customers collection: %w", err)
		}

		collection := core.NewBaseCollection("orders")

		// Customers can create and view their own orders; staff can see all
		collection.ListRule = types.Pointer(
			`customer.id = @request.auth.id || @request.auth.collectionName = "users"`,
		)
		collection.ViewRule = types.Pointer(
			`customer.id = @request.auth.id || @request.auth.collectionName = "users"`,
		)
		// Customers create their own orders via the custom /api/orders route (not direct PB API)
		// Direct collection create is staff-only; the route handler creates on their behalf
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
				Name:     "status",
				Required: true,
				Values:   []string{"pending", "confirmed", "delivered", "invoiced", "paid", "cancelled"},
			},
			&core.TextField{
				Name:     "notes",
				Required: false,
				Max:      2000,
			},
			// JSON snapshot of line items at time of order: [{variation_id, name, quantity, unit_price_cents}]
			&core.JSONField{
				Name:     "lineItems",
				Required: true,
			},
			&core.TextField{
				Name:     "squareOrderId",
				Required: false,
				Max:      100,
			},
			&core.TextField{
				Name:     "squareInvoiceId",
				Required: false,
				Max:      100,
			},
		)

		collection.AddIndex("idx_orders_customer", false, "customer", "")
		collection.AddIndex("idx_orders_status", false, "status", "")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
