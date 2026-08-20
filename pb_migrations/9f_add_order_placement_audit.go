package pb_migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return fmt.Errorf("find orders collection: %w", err)
		}
		collection.Fields.Add(
			&core.JSONField{Name: "placedBy", Required: false},
			&core.TextField{Name: "placementNote", Required: false, Max: 500, Hidden: true},
			&core.JSONField{Name: "editHistory", Required: false, Hidden: true},
		)
		// All writes must pass through validated custom routes.
		collection.UpdateRule = nil
		if err := app.Save(collection); err != nil {
			return fmt.Errorf("add order placement audit fields: %w", err)
		}

		records, err := app.FindAllRecords("orders")
		if err != nil {
			return fmt.Errorf("list orders for placement backfill: %w", err)
		}
		for _, record := range records {
			customer, err := app.FindRecordById("customers", record.GetString("customer"))
			if err != nil {
				return fmt.Errorf("find customer for order %s: %w", record.Id, err)
			}
			record.Set("placedBy", map[string]string{
				"type": "customer",
				"id":   customer.Id,
				"name": customer.GetString("name"),
			})
			if err := app.Save(record); err != nil {
				return fmt.Errorf("backfill order %s placement: %w", record.Id, err)
			}
		}
		return nil
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return err
		}
		collection.Fields.RemoveByName("editHistory")
		collection.Fields.RemoveByName("placementNote")
		collection.Fields.RemoveByName("placedBy")
		collection.UpdateRule = types.Pointer(`@request.auth.collectionName = "users"`)
		return app.Save(collection)
	})
}
