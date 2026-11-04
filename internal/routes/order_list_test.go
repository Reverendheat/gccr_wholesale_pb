package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func TestOrderListOptionsValidation(t *testing.T) {
	for _, raw := range []string{"page=0", "page=no", "page=100000000000", "sort=DROP", "status=bogus", "from=2026-02-30", "from=2026-03-02&to=2026-03-01", "invoices=maybe"} {
		values, _ := url.ParseQuery(raw)
		if _, err := parseOrderListOptions(values); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestStaffOrderListFiltersBeforePagination(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir(), HideStartBanner: true})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAppMigrations(); err != nil {
		t.Fatal(err)
	}
	companies, _ := app.FindCollectionByNameOrId("companies")
	company := core.NewRecord(companies)
	company.Set("name", "Ford")
	if err := app.Save(company); err != nil {
		t.Fatal(err)
	}
	customers, _ := app.FindCollectionByNameOrId("customers")
	customer := core.NewRecord(customers)
	customer.SetEmail("buyer@example.com")
	customer.SetPassword("test-password-12345")
	customer.Set("name", "Ford Buyer")
	customer.Set("squareCustomerId", "sq-ford")
	if err := app.Save(customer); err != nil {
		t.Fatal(err)
	}
	collection, _ := app.FindCollectionByNameOrId("orders")
	var oldest, largest string
	for i := 0; i < 103; i++ {
		rec := core.NewRecord(collection)
		rec.Set("customer", customer.Id)
		rec.Set("company", company.Id)
		rec.Set("status", "pending")
		rec.Set("lineItems", []map[string]any{{"variation_id": "v1", "quantity": 2, "unit_price_cents": i + 1}})
		if i == 0 {
			rec.Set("status", "invoiced")
			rec.Set("squareInvoiceId", "invoice-old")
		}
		if i == 1 {
			rec.Set("invoiceRecipients", []string{"buyer@example.com"})
		}
		if i >= 2 && i <= 4 {
			rec.Set("status", []string{"paid", "cancelled", "needs_review"}[i-2])
			rec.Set("squareInvoiceId", fmt.Sprintf("invoice-%d", i))
		}
		if i == 10 {
			rec.Set("fulfillment", map[string]any{"method": "delivery", "fee_cents": 10000})
		}
		if err := app.Save(rec); err != nil {
			t.Fatal(err)
		}
		date := fmt.Sprintf("2026-01-01 %02d:%02d:00.000Z", i/60, i%60)
		if _, err := app.DB().NewQuery("UPDATE orders SET created = {:date} WHERE id = {:id}").Bind(dbx.Params{"date": date, "id": rec.Id}).Execute(); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			oldest = rec.Id
		}
		if i == 10 {
			largest = rec.Id
		}
	}
	check := func(raw string, want int, more bool) []*core.Record {
		t.Helper()
		q, _ := url.ParseQuery(raw)
		opts, err := parseOrderListOptions(q)
		if err != nil {
			t.Fatal(err)
		}
		records, hasMore, err := findStaffOrderPage(app, opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != want || hasMore != more {
			t.Fatalf("%s: got %d, more %v; want %d, %v", raw, len(records), hasMore, want, more)
		}
		return records
	}
	first := check("", 25, true)
	second := check("page=2", 25, true)
	if first[0].GetString("created") <= second[0].GetString("created") {
		t.Fatal("pages are not newest first")
	}
	check("page=5", 3, false)
	invoice := check("invoices=true&status=unpaid", 1, false)
	if invoice[0].Id != oldest {
		t.Fatal("old unpaid invoice hidden")
	}
	check("invoices=true", 5, false)
	check("invoices=true&status=paid", 1, false)
	check("q=invoice-old", 1, false)
	check("q=Ford", 25, true)
	check("q=buyer@example.com", 25, true)
	check("q=%27+OR+1%3D1+--", 0, false)
	check("company="+company.Id, 25, true)
	check("company=missing", 0, false)
	check("from=2026-01-01&to=2026-01-01", 25, true)
	check("from=2026-01-02", 0, false)
	ascending := check("sort=oldest", 25, true)
	if ascending[0].Id != oldest {
		t.Fatal("oldest sort")
	}
	low := check("sort=total_asc", 25, true)
	if low[0].Id != oldest {
		t.Fatal("total sort")
	}
	high := check("sort=total_desc", 25, true)
	if high[0].Id != largest {
		t.Fatal("descending total sort must include delivery fees")
	}
	check("sort=company", 25, true)
}

func TestOrderListHandlerScopeAndMetadata(t *testing.T) {
	app, self, peer, outsider, staff := newCatalogAccessTestApp(t)
	companies, _ := app.FindCollectionByNameOrId("companies")
	company := core.NewRecord(companies)
	company.Set("name", "Ford")
	if err := app.Save(company); err != nil {
		t.Fatal(err)
	}
	self.Set("company", company.Id)
	if err := app.Save(self); err != nil {
		t.Fatal(err)
	}
	collection, _ := app.FindCollectionByNameOrId("orders")
	for _, buyer := range []*core.Record{self, peer, outsider} {
		rec := core.NewRecord(collection)
		rec.Set("customer", buyer.Id)
		rec.Set("status", "invoiced")
		rec.Set("squareInvoiceId", "private-square-id")
		rec.Set("lineItems", []map[string]any{{"variation_id": "v1", "quantity": 1}})
		if buyer.Id != outsider.Id {
			rec.Set("company", company.Id)
		}
		if err := app.Save(rec); err != nil {
			t.Fatal(err)
		}
	}
	response, err := invokeCatalogAccessHandler(t, app, self, http.MethodGet, "/orders?company=foreign&status=paid&page=2", nil, nil, handleListOrders())
	if err != nil {
		t.Fatal(err)
	}
	var customerResult struct {
		Orders []map[string]any `json:"orders"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &customerResult); err != nil {
		t.Fatal(err)
	}
	if len(customerResult.Orders) != 2 {
		t.Fatalf("customer scope changed: %s", response.Body)
	}
	for _, rec := range customerResult.Orders {
		if rec["customer"] == outsider.Id || rec["squareInvoiceId"] != "" || rec["expand"] != nil {
			t.Fatalf("customer data exposure: %+v", rec)
		}
	}
	response, err = invokeCatalogAccessHandler(t, app, staff, http.MethodGet, "/orders?invoices=true", nil, nil, handleListOrders())
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Orders  []map[string]any `json:"orders"`
		Page    int              `json:"page"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Orders) != 3 || result.Page != 1 || result.HasMore {
		t.Fatalf("unexpected staff page: %s", response.Body)
	}
	if _, err := invokeCatalogAccessHandler(t, app, staff, http.MethodGet, "/orders?sort=invalid", nil, nil, handleListOrders()); err == nil {
		t.Fatal("invalid query accepted")
	}
	if _, err := invokeCatalogAccessHandler(t, app, nil, http.MethodGet, "/orders", nil, nil, handleListOrders()); err == nil {
		t.Fatal("anonymous access allowed")
	}
}
