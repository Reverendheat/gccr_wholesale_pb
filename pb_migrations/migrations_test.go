package pb_migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func migrationPositions() map[string]int {
	positions := make(map[string]int)
	for i, migration := range core.AppMigrations.Items() {
		positions[migration.File] = i
	}
	return positions
}

func TestOrderStatusMigrationRunsAfterOrdersCreation(t *testing.T) {
	positions := migrationPositions()

	createOrders, ok := positions["4_create_orders.go"]
	if !ok {
		t.Fatal("orders creation migration is not registered")
	}

	addStatus, ok := positions["9b_add_needs_review_order_status.go"]
	if !ok {
		t.Fatal("needs_review status migration is not registered after orders creation")
	}

	if addStatus <= createOrders {
		t.Fatalf("needs_review migration position %d must follow orders creation position %d", addStatus, createOrders)
	}
}

func TestAccountAccessMigrationAppliesAccountRelationsAndRules(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir:  t.TempDir(),
		HideStartBanner: true,
	})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()

	if err := app.RunAppMigrations(); err != nil {
		t.Fatalf("run app migrations: %v", err)
	}

	for _, collectionName := range []string{"orders", "scheduledOrders"} {
		collection, err := app.FindCollectionByNameOrId(collectionName)
		if err != nil {
			t.Fatalf("find %s: %v", collectionName, err)
		}
		if collection.Fields.GetByName("company") == nil {
			t.Fatalf("%s.company relation missing", collectionName)
		}
		if collection.ListRule == nil || *collection.ListRule != accountReadRule {
			t.Fatalf("%s list rule = %v, want %q", collectionName, collection.ListRule, accountReadRule)
		}
		if collection.ViewRule == nil || *collection.ViewRule != accountReadRule {
			t.Fatalf("%s view rule = %v, want %q", collectionName, collection.ViewRule, accountReadRule)
		}
	}

	runner := core.NewMigrationsRunner(app, core.AppMigrations)
	accountAccessPosition := migrationPositions()["9c_add_account_access.go"]
	rollbackCount := len(core.AppMigrations.Items()) - accountAccessPosition
	if _, err := runner.Down(rollbackCount); err != nil {
		t.Fatalf("rollback through account access migration: %v", err)
	}
	for _, collectionName := range []string{"orders", "scheduledOrders"} {
		collection, err := app.FindCollectionByNameOrId(collectionName)
		if err != nil {
			t.Fatalf("find %s after rollback: %v", collectionName, err)
		}
		if collection.Fields.GetByName("company") != nil {
			t.Fatalf("%s.company relation remains after rollback", collectionName)
		}
	}
}

func TestFulfillmentMigrationAddsOrderAndScheduleSnapshots(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir:  t.TempDir(),
		HideStartBanner: true,
	})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()

	if err := app.RunAppMigrations(); err != nil {
		t.Fatalf("run app migrations: %v", err)
	}

	for _, collectionName := range []string{"orders", "scheduledOrders"} {
		collection, err := app.FindCollectionByNameOrId(collectionName)
		if err != nil {
			t.Fatalf("find %s: %v", collectionName, err)
		}
		if collection.Fields.GetByName("fulfillment") == nil {
			t.Fatalf("%s.fulfillment snapshot missing", collectionName)
		}
	}
}

func TestAccountAccessMigrationRunsAfterCompanyCreation(t *testing.T) {
	positions := migrationPositions()

	createCompanies, ok := positions["7_create_companies.go"]
	if !ok {
		t.Fatal("companies creation migration is not registered")
	}

	accountAccess, ok := positions["9c_add_account_access.go"]
	if !ok {
		t.Fatal("account access migration is not registered")
	}

	if accountAccess <= createCompanies {
		t.Fatalf("account access migration position %d must follow companies creation position %d", accountAccess, createCompanies)
	}
}
