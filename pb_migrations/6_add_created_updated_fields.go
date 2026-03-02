package pb_migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// addTimestampFields appends created/updated AutodateFields to a collection
// and syncs the schema. Existing rows are backfilled to the current time.
func addTimestampFields(app core.App, collectionName string) error {
	col, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return err
	}

	col.Fields.Add(
		&core.AutodateField{
			Name:     "created",
			System:   true,
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			System:   true,
			OnCreate: true,
			OnUpdate: true,
		},
	)

	if err := app.Save(col); err != nil {
		return err
	}

	// Backfill existing rows so they show a real timestamp instead of "—".
	_, err = app.DB().NewQuery(
		"UPDATE `" + collectionName + "` SET created = strftime('%Y-%m-%d %H:%M:%f', 'now') || 'Z', updated = strftime('%Y-%m-%d %H:%M:%f', 'now') || 'Z' WHERE created = ''",
	).Execute()
	return err
}

func init() {
	m.Register(func(app core.App) error {
		for _, name := range []string{"customers", "orders", "scheduled_orders"} {
			if err := addTimestampFields(app, name); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		for _, name := range []string{"customers", "orders", "scheduled_orders"} {
			col, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			col.Fields.RemoveByName("created")
			col.Fields.RemoveByName("updated")
			if err := app.Save(col); err != nil {
				return err
			}
		}
		return nil
	})
}
