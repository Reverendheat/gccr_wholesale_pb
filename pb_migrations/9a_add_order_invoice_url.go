package pb_migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		orders, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return err
		}

		orders.Fields.Add(&core.URLField{
			Name:     "squareInvoiceUrl",
			Required: false,
		})

		return app.Save(orders)
	}, func(app core.App) error {
		orders, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return err
		}

		orders.Fields.RemoveByName("squareInvoiceUrl")
		return app.Save(orders)
	})
}
