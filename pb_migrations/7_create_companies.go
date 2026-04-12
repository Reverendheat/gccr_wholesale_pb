package pb_migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// --- 1. Create the companies collection ---
		companies := core.NewBaseCollection("companies")

		companies.ListRule = types.Pointer(`@request.auth.collectionName = "users"`)
		companies.ViewRule = types.Pointer(
			`@request.auth.collectionName = "users" || @request.auth.company = id`,
		)
		companies.CreateRule = nil
		companies.UpdateRule = types.Pointer(`@request.auth.collectionName = "users"`)
		companies.DeleteRule = nil

		companies.Fields.Add(
			&core.TextField{
				Name:     "name",
				Required: true,
				Max:      200,
			},
			&core.TextField{
				Name:     "address",
				Required: false,
				Max:      500,
			},
			&core.TextField{
				Name:     "phone",
				Required: false,
				Max:      30,
			},
			&core.TextField{
				Name:     "email",
				Required: false,
				Max:      200,
			},
		)

		companies.AddIndex("idx_companies_name", false, "name", "")

		if err := app.Save(companies); err != nil {
			return err
		}

		// --- 2. Add a company relation to the customers collection ---
		customers, err := app.FindCollectionByNameOrId("customers")
		if err != nil {
			return err
		}

		customers.Fields.Add(&core.RelationField{
			Name:         "company",
			CollectionId: companies.Id,
			Required:     false,
			MaxSelect:    1,
		})

		return app.Save(customers)
	}, func(app core.App) error {
		// Remove the company field from customers first
		customers, err := app.FindCollectionByNameOrId("customers")
		if err == nil {
			customers.Fields.RemoveByName("company")
			_ = app.Save(customers)
		}

		companies, err := app.FindCollectionByNameOrId("companies")
		if err != nil {
			return err
		}
		return app.Delete(companies)
	})
}
