package pb_migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		for _, collectionName := range []string{"orders", "scheduledOrders"} {
			collection, err := app.FindCollectionByNameOrId(collectionName)
			if err != nil {
				return fmt.Errorf("find %s collection: %w", collectionName, err)
			}
			collection.Fields.Add(&core.JSONField{
				Name:     "fulfillment",
				Required: false,
			})
			if err := app.Save(collection); err != nil {
				return fmt.Errorf("add fulfillment to %s: %w", collectionName, err)
			}
		}
		return nil
	}, func(app core.App) error {
		for _, collectionName := range []string{"scheduledOrders", "orders"} {
			collection, err := app.FindCollectionByNameOrId(collectionName)
			if err != nil {
				return err
			}
			collection.Fields.RemoveByName("fulfillment")
			if err := app.Save(collection); err != nil {
				return err
			}
		}
		return nil
	})
}
