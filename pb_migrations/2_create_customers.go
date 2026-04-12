package pb_migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewAuthCollection("customers")

		// Customers can only view their own record; staff (users collection) can list/view all
		collection.ListRule = types.Pointer(`@request.auth.collectionName = "users"`)
		collection.ViewRule = types.Pointer(`id = @request.auth.id || @request.auth.collectionName = "users"`)
		// Only superusers can create/update/delete customer records
		collection.CreateRule = nil
		collection.UpdateRule = nil
		collection.DeleteRule = nil

		collection.Fields.Add(
			&core.TextField{
				Name:     "name",
				Required: true,
				Max:      200,
			},
			&core.TextField{
				Name:     "phone",
				Required: false,
				Max:      30,
			},
			&core.TextField{
				Name:     "squareCustomerId",
				Required: true,
				Max:      100,
			},
		)

		collection.AddIndex("idx_customers_square_id", true, "squareCustomerId", "")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("customers")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
