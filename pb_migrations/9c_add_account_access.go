package pb_migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

const accountReadRule = `customer.id = @request.auth.id || (company != "" && company = @request.auth.company) || @request.auth.collectionName = "users"`

func init() {
	m.Register(func(app core.App) error {
		companies, err := app.FindCollectionByNameOrId("companies")
		if err != nil {
			return fmt.Errorf("find companies collection: %w", err)
		}

		for _, collectionName := range []string{"orders", "scheduledOrders"} {
			collection, err := app.FindCollectionByNameOrId(collectionName)
			if err != nil {
				return fmt.Errorf("find %s collection: %w", collectionName, err)
			}

			collection.Fields.Add(&core.RelationField{
				Name:         "company",
				CollectionId: companies.Id,
				Required:     false,
				MaxSelect:    1,
			})
			collection.ListRule = types.Pointer(accountReadRule)
			collection.ViewRule = types.Pointer(accountReadRule)
			collection.AddIndex("idx_"+collectionName+"_company", false, "company", "")

			if err := app.Save(collection); err != nil {
				return fmt.Errorf("save %s account schema: %w", collectionName, err)
			}
		}

		// Snapshot existing customer account assignments onto historical records.
		// Unassigned records remain visible only to their creator.
		for _, collectionName := range []string{"orders", "scheduledOrders"} {
			records, err := app.FindAllRecords(collectionName)
			if err != nil {
				return fmt.Errorf("list %s for account backfill: %w", collectionName, err)
			}
			for _, record := range records {
				if record.GetString("company") != "" {
					continue
				}
				customer, err := app.FindRecordById("customers", record.GetString("customer"))
				if err != nil {
					continue
				}
				if companyID := customer.GetString("company"); companyID != "" {
					record.Set("company", companyID)
					if err := app.Save(record); err != nil {
						return fmt.Errorf("backfill %s %s company: %w", collectionName, record.Id, err)
					}
				}
			}
		}

		return nil
	}, func(app core.App) error {
		legacyReadRule := `customer.id = @request.auth.id || @request.auth.collectionName = "users"`
		for _, collectionName := range []string{"scheduledOrders", "orders"} {
			collection, err := app.FindCollectionByNameOrId(collectionName)
			if err != nil {
				return err
			}
			collection.RemoveIndex("idx_" + collectionName + "_company")
			collection.Fields.RemoveByName("company")
			collection.ListRule = types.Pointer(legacyReadRule)
			collection.ViewRule = types.Pointer(legacyReadRule)
			if err := app.Save(collection); err != nil {
				return err
			}
		}
		return nil
	})
}
