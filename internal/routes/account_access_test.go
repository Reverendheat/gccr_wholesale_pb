package routes

import (
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	_ "github.com/reverendheat/gccr_invoice/pb_migrations"
	squaresdk "github.com/square/square-go-sdk/v3"
)

func TestCustomerRecordFilterOwnOnlyWithoutAccount(t *testing.T) {
	got := customerRecordFilter("customer1", "", false)
	want := "customer = 'customer1'"
	if got != want {
		t.Fatalf("customerRecordFilter() = %q, want %q", got, want)
	}
}

func TestCustomerRecordFilterIncludesAccountAndOwner(t *testing.T) {
	got := customerRecordFilter("customer1", "company1", false)
	want := "(customer = 'customer1' || company = 'company1')"
	if got != want {
		t.Fatalf("customerRecordFilter() = %q, want %q", got, want)
	}
}

func TestCustomerRecordFilterCanRequireActiveRecords(t *testing.T) {
	got := customerRecordFilter("customer1", "company1", true)
	want := "(customer = 'customer1' || company = 'company1') && active = true"
	if got != want {
		t.Fatalf("customerRecordFilter() = %q, want %q", got, want)
	}
}

func TestCustomerCanEditOnlyOwnPendingOrder(t *testing.T) {
	tests := []struct {
		name       string
		customerID string
		ownerID    string
		status     string
		actorType  string
		want       bool
	}{
		{name: "own pending order", customerID: "customer1", ownerID: "customer1", status: "pending", actorType: "customer", want: true},
		{name: "legacy own pending order", customerID: "customer1", ownerID: "customer1", status: "pending", want: true},
		{name: "staff-created pending order", customerID: "customer1", ownerID: "customer1", status: "pending", actorType: "staff", want: false},
		{name: "account peer pending order", customerID: "customer1", ownerID: "customer2", status: "pending", actorType: "customer", want: false},
		{name: "own confirmed order", customerID: "customer1", ownerID: "customer1", status: "confirmed", actorType: "customer", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := customerCanEditOrder(tt.customerID, tt.ownerID, tt.status, tt.actorType); got != tt.want {
				t.Fatalf("customerCanEditOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCustomerCanCancelOnlyOwnPendingPreSquareOrder(t *testing.T) {
	tests := []struct {
		name            string
		customerID      string
		ownerID         string
		status          string
		squareOrderID   string
		squareInvoiceID string
		want            bool
	}{
		{name: "own pending order", customerID: "customer1", ownerID: "customer1", status: "pending", want: true},
		{name: "account peer order", customerID: "customer1", ownerID: "customer2", status: "pending", want: false},
		{name: "own confirmed order", customerID: "customer1", ownerID: "customer1", status: "confirmed", want: false},
		{name: "Square order exists", customerID: "customer1", ownerID: "customer1", status: "pending", squareOrderID: "SQ1", want: false},
		{name: "Square invoice exists", customerID: "customer1", ownerID: "customer1", status: "pending", squareInvoiceID: "INV1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := customerCanCancelOrder(tt.customerID, tt.ownerID, tt.status, tt.squareOrderID, tt.squareInvoiceID)
			if got != tt.want {
				t.Fatalf("customerCanCancelOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateAccountSelection(t *testing.T) {
	tests := []struct {
		name           string
		companyID      string
		newCompanyName string
		wantError      bool
	}{
		{name: "existing account", companyID: "company1"},
		{name: "new account", newCompanyName: "Smith Foods LLC"},
		{name: "missing account", wantError: true},
		{name: "ambiguous account", companyID: "company1", newCompanyName: "Smith Foods LLC", wantError: true},
		{name: "blank new account", newCompanyName: "   ", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAccountSelection(tt.companyID, tt.newCompanyName)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateAccountSelection() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestNormalizeCompanyNameForSuggestion(t *testing.T) {
	if got, want := normalizeCompanyName("  Smith   Foods LLC "), "smith foods llc"; got != want {
		t.Fatalf("normalizeCompanyName() = %q, want %q", got, want)
	}
}

func TestAssignCustomerCompanyBackfillsUnassignedHistoryOnly(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir(), HideStartBanner: true})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAppMigrations(); err != nil {
		t.Fatal(err)
	}

	companies, _ := app.FindCollectionByNameOrId("companies")
	firstCompany := core.NewRecord(companies)
	firstCompany.Set("name", "First Account")
	if err := app.Save(firstCompany); err != nil {
		t.Fatal(err)
	}
	secondCompany := core.NewRecord(companies)
	secondCompany.Set("name", "Second Account")
	if err := app.Save(secondCompany); err != nil {
		t.Fatal(err)
	}

	customers, _ := app.FindCollectionByNameOrId("customers")
	customer := core.NewRecord(customers)
	customer.SetEmail("customer@example.com")
	customer.SetPassword("test-password-12345")
	customer.Set("name", "Customer")
	customer.Set("squareCustomerId", "square-customer-1")
	if err := app.Save(customer); err != nil {
		t.Fatal(err)
	}

	orders, _ := app.FindCollectionByNameOrId("orders")
	order := core.NewRecord(orders)
	order.Set("customer", customer.Id)
	order.Set("status", "pending")
	order.Set("lineItems", []map[string]any{{"variation_id": "variation1", "quantity": 1}})
	if err := app.Save(order); err != nil {
		t.Fatal(err)
	}

	schedules, _ := app.FindCollectionByNameOrId("scheduledOrders")
	schedule := core.NewRecord(schedules)
	schedule.Set("customer", customer.Id)
	schedule.Set("frequency", "weekly")
	schedule.Set("lineItems", []map[string]any{{"variation_id": "variation1", "quantity": 1}})
	schedule.Set("next_run_at", "2030-01-01 00:00:00.000Z")
	schedule.Set("active", true)
	if err := app.Save(schedule); err != nil {
		t.Fatal(err)
	}

	if err := assignCustomerCompany(app, customer, firstCompany.Id); err != nil {
		t.Fatal(err)
	}
	order, _ = app.FindRecordById("orders", order.Id)
	schedule, _ = app.FindRecordById("scheduledOrders", schedule.Id)
	if order.GetString("company") != firstCompany.Id || schedule.GetString("company") != firstCompany.Id {
		t.Fatal("first assignment did not backfill unassigned history")
	}

	if err := assignCustomerCompany(app, customer, secondCompany.Id); err != nil {
		t.Fatal(err)
	}
	order, _ = app.FindRecordById("orders", order.Id)
	schedule, _ = app.FindRecordById("scheduledOrders", schedule.Id)
	if order.GetString("company") != firstCompany.Id || schedule.GetString("company") != firstCompany.Id {
		t.Fatal("reassignment moved historical records")
	}
}

func TestFindSuggestedAccountsReturnsNonNilEmptySliceWithoutCompanyName(t *testing.T) {
	got, err := findSuggestedAccounts(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("suggested accounts must encode as [] instead of null")
	}
	if len(got) != 0 {
		t.Fatalf("suggested accounts = %+v, want empty", got)
	}
}

func TestCustomerDetailsIncludesSquareCompanyName(t *testing.T) {
	details := customerDetails(&squaresdk.Customer{
		ID:           squaresdk.String("square1"),
		GivenName:    squaresdk.String("Jane"),
		FamilyName:   squaresdk.String("Smith"),
		EmailAddress: squaresdk.String("jane@example.com"),
		PhoneNumber:  squaresdk.String("555-0100"),
		CompanyName:  squaresdk.String("Smith Foods LLC"),
	}, "fallback@example.com")

	if details.CompanyName != "Smith Foods LLC" {
		t.Fatalf("company name = %q", details.CompanyName)
	}
	if details.Name != "Jane Smith" || details.ID != "square1" {
		t.Fatalf("unexpected customer details: %+v", details)
	}
}

func TestCanCancelScheduledOrder(t *testing.T) {
	tests := []struct {
		name       string
		collection string
		authID     string
		creatorID  string
		want       bool
	}{
		{name: "creator", collection: "customers", authID: "customer1", creatorID: "customer1", want: true},
		{name: "account peer", collection: "customers", authID: "customer2", creatorID: "customer1", want: false},
		{name: "staff", collection: "users", authID: "staff1", creatorID: "customer1", want: true},
		{name: "unknown auth collection", collection: "other", authID: "other1", creatorID: "customer1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canCancelScheduledOrder(tt.collection, tt.authID, tt.creatorID); got != tt.want {
				t.Fatalf("canCancelScheduledOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}
