package pb_migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		orders, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return err
		}

		field := orders.Fields.GetByName("status")
		selectField, ok := field.(*core.SelectField)
		if !ok {
			return nil
		}

		hasNeedsReview := false
		for _, value := range selectField.Values {
			if value == "needs_review" {
				hasNeedsReview = true
				break
			}
		}
		if !hasNeedsReview {
			selectField.Values = append(selectField.Values, "needs_review")
		}
		orders.UpdateRule = nil
		return app.Save(orders)
	}, func(app core.App) error {
		orders, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return err
		}

		field := orders.Fields.GetByName("status")
		selectField, ok := field.(*core.SelectField)
		if !ok {
			return nil
		}

		values := make([]string, 0, len(selectField.Values))
		for _, value := range selectField.Values {
			if value != "needs_review" {
				values = append(values, value)
			}
		}
		selectField.Values = values
		orders.UpdateRule = types.Pointer(`@request.auth.collectionName = "users"`)
		return app.Save(orders)
	})
}
