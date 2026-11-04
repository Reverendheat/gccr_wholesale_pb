package routes

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const orderPageSize = 25

type orderListOptions struct {
	Page                                    int
	Sort, Status, Company, Search, From, To string
	Invoices                                bool
}

func parseOrderListOptions(q url.Values) (orderListOptions, error) {
	o := orderListOptions{Page: 1, Sort: q.Get("sort"), Status: q.Get("status"), Company: q.Get("company"), Search: strings.TrimSpace(q.Get("q")), From: q.Get("from"), To: q.Get("to")}
	if s := q.Get("page"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 1000000 {
			return o, fmt.Errorf("invalid page")
		}
		o.Page = n
	}
	switch o.Sort {
	case "", "newest", "oldest", "company", "total_asc", "total_desc":
	default:
		return o, fmt.Errorf("invalid sort")
	}
	switch o.Status {
	case "", "pending", "confirmed", "delivered", "invoiced", "paid", "cancelled", "needs_review", "unpaid":
	default:
		return o, fmt.Errorf("invalid status")
	}
	if s := q.Get("invoices"); s != "" {
		if s != "true" && s != "false" {
			return o, fmt.Errorf("invalid invoices filter")
		}
		o.Invoices = s == "true"
	}
	for _, date := range []string{o.From, o.To} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return o, fmt.Errorf("invalid date; use YYYY-MM-DD")
			}
		}
	}
	if o.From != "" && o.To != "" && o.From > o.To {
		return o, fmt.Errorf("from date must not follow to date")
	}
	if utf8.RuneCountInString(o.Search) > 200 {
		return o, fmt.Errorf("search must be 200 characters or fewer")
	}
	return o, nil
}

func findStaffOrderPage(app core.App, o orderListOptions) ([]*core.Record, bool, error) {
	query := app.RecordQuery("orders")
	if o.Invoices {
		query.AndWhere(dbx.NewExp("(orders.squareInvoiceId != '' OR COALESCE(json_array_length(orders.invoiceRecipients), 0) > 0)"))
	}
	if o.Status == "unpaid" {
		query.AndWhere(dbx.NewExp("orders.status = 'invoiced' AND orders.squareInvoiceId != ''"))
	} else if o.Status != "" {
		query.AndWhere(dbx.HashExp{"orders.status": o.Status})
	}
	if o.Company != "" {
		query.AndWhere(dbx.HashExp{"orders.company": o.Company})
	}
	if o.Search != "" {
		query.AndWhere(dbx.NewExp(`(instr(lower(orders.id), lower({:q})) > 0
   OR instr(lower(orders.squareOrderId), lower({:q})) > 0
   OR instr(lower(orders.squareInvoiceId), lower({:q})) > 0
   OR orders.customer IN (SELECT id FROM customers WHERE instr(lower(name), lower({:q})) > 0 OR instr(lower(email), lower({:q})) > 0)
   OR orders.company IN (SELECT id FROM companies WHERE instr(lower(name), lower({:q})) > 0))`, dbx.Params{"q": o.Search}))
	}
	if o.From != "" {
		query.AndWhere(dbx.NewExp("orders.created >= {:from}", dbx.Params{"from": o.From + " 00:00:00.000Z"}))
	}
	if o.To != "" {
		end, _ := time.Parse("2006-01-02", o.To)
		query.AndWhere(dbx.NewExp("orders.created < {:to}", dbx.Params{"to": end.AddDate(0, 0, 1).Format("2006-01-02") + " 00:00:00.000Z"}))
	}
	sort := []string{"orders.created DESC", "orders.id DESC"}
	switch o.Sort {
	case "oldest":
		sort = []string{"orders.created ASC", "orders.id ASC"}
	case "company":
		sort = append([]string{"COALESCE((SELECT name FROM companies WHERE companies.id = orders.company), '') COLLATE NOCASE ASC"}, sort...)
	case "total_asc", "total_desc":
		direction := "ASC"
		if o.Sort == "total_desc" {
			direction = "DESC"
		}
		sort = append([]string{`(COALESCE((SELECT SUM(COALESCE(json_extract(value, '$.quantity'), 0) * COALESCE(json_extract(value, '$.unit_price_cents'), 0)) FROM json_each(orders.lineItems)), 0) + COALESCE(json_extract(orders.fulfillment, '$.fee_cents'), 0)) ` + direction}, sort...)
	}
	records := []*core.Record{}
	err := query.OrderBy(sort...).Limit(orderPageSize + 1).Offset(int64((o.Page - 1) * orderPageSize)).All(&records)
	more := len(records) > orderPageSize
	if more {
		records = records[:orderPageSize]
	}
	return records, more, err
}
