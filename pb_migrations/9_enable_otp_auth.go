package pb_migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		for _, name := range []string{"users", "customers"} {
			collection, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}

			collection.PasswordAuth.Enabled = false
			collection.OTP.Enabled = true

			if err := app.Save(collection); err != nil {
				return err
			}
		}

		return nil
	}, func(app core.App) error {
		for _, name := range []string{"users", "customers"} {
			collection, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}

			collection.PasswordAuth.Enabled = true
			collection.OTP.Enabled = false

			if err := app.Save(collection); err != nil {
				return err
			}
		}

		return nil
	})
}
