package pb_migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestOrderStatusMigrationRunsAfterOrdersCreation(t *testing.T) {
	positions := make(map[string]int)
	for i, migration := range core.AppMigrations.Items() {
		positions[migration.File] = i
	}

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
