package pb_migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/dbutils"
)

func setAuthEmailCollation(app core.App, collation string) error {
	for _, collectionName := range []string{"users", "customers"} {
		collection, err := app.FindCollectionByNameOrId(collectionName)
		if err != nil {
			return fmt.Errorf("find %s collection: %w", collectionName, err)
		}

		emailIndex, ok := dbutils.FindSingleColumnUniqueIndex(collection.Indexes, core.FieldNameEmail)
		if !ok {
			return fmt.Errorf("find %s email index", collectionName)
		}
		emailIndex.Columns[0].Collate = collation
		collection.RemoveIndex(emailIndex.IndexName)
		collection.Indexes = append(collection.Indexes, emailIndex.Build())

		if err := app.Save(collection); err != nil {
			return fmt.Errorf("save %s email index: %w", collectionName, err)
		}
	}
	return nil
}

func init() {
	m.Register(func(app core.App) error {
		return setAuthEmailCollation(app, "NOCASE")
	}, func(app core.App) error {
		return setAuthEmailCollation(app, "")
	})
}
