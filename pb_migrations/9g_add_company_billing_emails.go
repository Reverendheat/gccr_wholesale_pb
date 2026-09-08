package pb_migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		companies, err := app.FindCollectionByNameOrId("companies")
		if err != nil {
			return err
		}
		companies.Fields.Add(&core.JSONField{Name: "billingEmails", MaxSize: 64 * 1024})
		companies.UpdateRule = types.Pointer(`@request.auth.collectionName = "users"`)
		if err := app.Save(companies); err != nil {
			return err
		}
		orders, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return err
		}
		orders.Fields.Add(
			&core.JSONField{Name: "invoiceRecipients", MaxSize: 64 * 1024},
			&core.JSONField{Name: "invoiceSentRecipients", MaxSize: 64 * 1024},
			&core.TextField{Name: "invoiceEmailError"},
			&core.DateField{Name: "invoiceEmailSentAt"},
			// Freeze the create payload before contacting Square so retries use
			// the same due date and original customer with the same idempotency key.
			&core.JSONField{Name: "invoiceDraftRequest", Hidden: true, MaxSize: 4096},
		)
		return app.Save(orders)
	}, func(app core.App) error {
		orders, err := app.FindCollectionByNameOrId("orders")
		if err != nil {
			return err
		}
		for _, name := range []string{"invoiceRecipients", "invoiceSentRecipients", "invoiceEmailError", "invoiceEmailSentAt", "invoiceDraftRequest"} {
			orders.Fields.RemoveByName(name)
		}
		if err := app.Save(orders); err != nil {
			return err
		}
		companies, err := app.FindCollectionByNameOrId("companies")
		if err != nil {
			return err
		}
		companies.Fields.RemoveByName("billingEmails")
		return app.Save(companies)
	})
}
