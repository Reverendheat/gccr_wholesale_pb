package routes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/reverendheat/gccr_invoice/internal/hooks"
	"github.com/reverendheat/gccr_invoice/internal/square"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func invoiceTestCompany(t *testing.T, app core.App, name string, emails []string) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("companies")
	if err != nil {
		t.Fatal(err)
	}
	company := core.NewRecord(collection)
	company.Set("name", name)
	company.Set("billingEmails", emails)
	if err := app.Save(company); err != nil {
		t.Fatal(err)
	}
	return company
}

func invoiceTestOrder(t *testing.T, app core.App, customer, company *core.Record) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("orders")
	if err != nil {
		t.Fatal(err)
	}
	order := core.NewRecord(collection)
	order.Set("customer", customer.Id)
	if company != nil {
		order.Set("company", company.Id)
	}
	order.Set("status", "confirmed")
	order.Set("squareOrderId", "SQ_ORDER")
	order.Set("lineItems", []map[string]any{{"variation_id": "VAR1", "quantity": 1, "unit_price_cents": 1000, "currency": "USD"}})
	if err := app.Save(order); err != nil {
		t.Fatal(err)
	}
	return order
}

func TestInvoiceRecipientsUseHistoricalCompanyAndAccountEmail(t *testing.T) {
	app, customer, _, _, _ := newCatalogAccessTestApp(t)
	company := invoiceTestCompany(t, app, "Original", []string{"billing@example.com"})
	current := invoiceTestCompany(t, app, "Current", []string{"wrong@example.com"})
	customer.Set("company", current.Id)
	if err := app.Save(customer); err != nil {
		t.Fatal(err)
	}
	order := invoiceTestOrder(t, app, customer, company)
	got, err := selectedInvoiceRecipients(app, order)
	if err != nil || !reflect.DeepEqual(got, []string{"billing@example.com"}) {
		t.Fatalf("historical recipients = %v, %v", got, err)
	}
	company.Set("billingEmails", []string{})
	company.Set("email", "not-the-buyer@example.com")
	if err := app.Save(company); err != nil {
		t.Fatal(err)
	}
	got, err = selectedInvoiceRecipients(app, order)
	if err != nil || !reflect.DeepEqual(got, []string{customer.Email()}) {
		t.Fatalf("fallback recipients = %v, %v", got, err)
	}
	order.Set("company", "missing-company")
	if _, err := selectedInvoiceRecipients(app, order); err == nil {
		t.Fatal("missing historical company silently fell back")
	}
	company.Set("billingEmails", `["invalid"]`)
	if err := app.Save(company); err != nil {
		t.Fatal(err)
	}
	order.Set("company", company.Id)
	if _, err := selectedInvoiceRecipients(app, order); err == nil {
		t.Fatal("invalid configuration silently fell back")
	}
}

func TestBillingConfigurationAuthorizationAndRawCollectionValidation(t *testing.T) {
	app, customer, _, _, staff := newCatalogAccessTestApp(t)
	hooks.Register(app, nil)
	company := invoiceTestCompany(t, app, "Account", nil)
	customer.Set("company", company.Id)
	if err := app.Save(customer); err != nil {
		t.Fatal(err)
	}
	order := invoiceTestOrder(t, app, customer, company)
	for _, auth := range []*core.Record{nil, customer} {
		_, err := invokeCatalogAccessHandler(t, app, auth, "PATCH", "/billing", map[string]any{"billing_emails": []string{"a@example.com"}}, map[string]string{"id": company.Id}, handleUpdateCompanyBilling())
		if router.ToApiError(err).Status != http.StatusForbidden {
			t.Fatalf("billing authorization error = %v", err)
		}
		_, err = invokeCatalogAccessHandler(t, app, auth, "GET", "/invoice-delivery", nil, map[string]string{"id": order.Id}, handleInvoiceDelivery())
		if router.ToApiError(err).Status != http.StatusForbidden {
			t.Fatalf("preview authorization error = %v", err)
		}
		_, err = invokeCatalogAccessHandler(t, app, auth, "POST", "/invoices", map[string]any{"order_id": order.Id, "recipients": []string{"a@example.com"}}, nil, handleSendInvoice(nil, "LOC"))
		if router.ToApiError(err).Status != http.StatusForbidden {
			t.Fatalf("send authorization error = %v", err)
		}
	}
	pbRouter, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := pbRouter.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	patch := func(auth *core.Record, raw string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/collections/companies/records/"+company.Id, strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if auth != nil {
			token, err := auth.NewAuthToken()
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response.Code
	}
	if code := patch(customer, `{"billingEmails":["bad@example.com"]}`); code < 400 {
		t.Fatalf("customer raw write accepted: %d", code)
	}
	if code := patch(nil, `{"billingEmails":["bad@example.com"]}`); code < 400 {
		t.Fatalf("anonymous raw write accepted: %d", code)
	}
	for _, value := range []string{`"a@example.com"`, `"[]"`, `"[\"a@example.com\"]"`, `[null]`, `[7]`, `["not-an-email"]`, `{}`, `true`} {
		if code := patch(staff, `{"billingEmails":`+value+`}`); code != 400 {
			t.Fatalf("raw invalid %s returned %d", value, code)
		}
	}
	if code := patch(staff, `{"billingEmails":[" Accounts@Example.com ","accounts@example.com"]}`); code != 200 {
		t.Fatalf("valid staff patch returned %d", code)
	}
	company, err = app.FindRecordById("companies", company.Id)
	if err != nil {
		t.Fatal(err)
	}
	emails, err := recordEmailList(company, "billingEmails")
	if err != nil || !reflect.DeepEqual(emails, []string{"accounts@example.com"}) {
		t.Fatalf("stored normalized emails = %v, %v", emails, err)
	}
	company.Set("billingEmails", `["invalid"]`)
	if err := app.Save(company); err == nil {
		t.Fatal("model save bypassed email validation")
	}
}

type fakeInvoiceSquare struct {
	mu                      sync.Mutex
	status                  string
	url                     string
	creates                 int
	publishes               int
	gets                    int
	publishFailures         int
	createFailures          int
	publishCommittedFailure bool
	createPayloads          [][]byte
}

func newInvoiceSquare(t *testing.T) (*square.Client, *fakeInvoiceSquare) {
	t.Helper()
	fake := &fakeInvoiceSquare{status: "DRAFT", url: "https://square.example/invoice/initial"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v2/orders/SQ_ORDER":
			fmt.Fprint(w, `{"order":{"id":"SQ_ORDER","location_id":"ORIGINAL_LOC","customer_id":"ORIGINAL_CUSTOMER"}}`)
			return
		case r.Method == "POST" && r.URL.Path == "/v2/invoices":
			fake.creates++
			var body json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode create: %v", err)
			}
			fake.createPayloads = append(fake.createPayloads, append([]byte(nil), body...))
			if fake.createFailures > 0 {
				fake.createFailures--
				// An incomplete successful response is ambiguous: Square may
				// already have committed this same idempotent create.
				fmt.Fprint(w, `{}`)
				return
			}
		case r.Method == "POST" && r.URL.Path == "/v2/invoices/INV1/publish":
			fake.publishes++
			if fake.publishFailures > 0 {
				fake.publishFailures--
				if fake.publishCommittedFailure {
					fake.status = "UNPAID"
				}
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"BAD_REQUEST","detail":"publication unavailable"}]}`)
				return
			}
			fake.status = "UNPAID"
		case r.Method == "GET" && r.URL.Path == "/v2/invoices/INV1":
			fake.gets++
		default:
			t.Errorf("unexpected Square request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"invoice": map[string]any{
			"id": "INV1", "version": 0, "status": fake.status, "order_id": "SQ_ORDER", "delivery_method": "SHARE_MANUALLY", "public_url": fake.url,
		}})
	}))
	t.Cleanup(server.Close)
	return &square.Client{SDK: squareclient.NewClient(option.WithToken("test"), option.WithBaseURL(server.URL))}, fake
}

func postInvoice(t *testing.T, app core.App, staff, order *core.Record, sq *square.Client, recipients []string) map[string]json.RawMessage {
	t.Helper()
	response, err := invokeCatalogAccessHandler(t, app, staff, "POST", "/invoices", map[string]any{"order_id": order.Id, "recipients": recipients}, nil, handleSendInvoice(sq, "CURRENT_LOC"))
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("invoice response %d: %s", response.Code, response.Body.String())
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestInvoicePartialMailFailureRetriesOnlyPendingRecipients(t *testing.T) {
	app, customer, _, _, staff := newCatalogAccessTestApp(t)
	company := invoiceTestCompany(t, app, "Account", []string{"first@example.com", "second@example.com"})
	order := invoiceTestOrder(t, app, customer, company)
	sq, fake := newInvoiceSquare(t)
	var submitted []string
	var links []string
	failSecond := true
	app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error {
		if len(e.Message.To) != 1 || len(e.Message.Cc) != 0 || len(e.Message.Bcc) != 0 {
			t.Fatal("recipient addresses were exposed together")
		}
		recipient := e.Message.To[0].Address
		if recipient == "second@example.com" && failSecond {
			return errors.New("SMTP unavailable")
		}
		submitted = append(submitted, recipient)
		links = append(links, e.Message.HTML)
		return nil
	})
	beforeSend := time.Now().AddDate(0, 0, 15).Format("2006-01-02")
	result := postInvoice(t, app, staff, order, sq, []string{"first@example.com", "second@example.com"})
	afterSend := time.Now().AddDate(0, 0, 15).Format("2006-01-02")
	if string(result["notification_sent"]) != "false" {
		t.Fatalf("false success: %s", result["notification_sent"])
	}
	order, _ = app.FindRecordById("orders", order.Id)
	delivery, err := getInvoiceDelivery(app, order)
	if err != nil || order.GetString("status") != "invoiced" || order.GetString("squareInvoiceId") != "INV1" || !reflect.DeepEqual(delivery.PendingRecipients, []string{"second@example.com"}) || delivery.Error == "" {
		t.Fatalf("partial delivery not durable: %+v, %v, order=%v", delivery, err, order)
	}
	var create struct {
		Invoice struct {
			DeliveryMethod   string `json:"delivery_method"`
			PrimaryRecipient struct {
				CustomerID string `json:"customer_id"`
			} `json:"primary_recipient"`
			LocationID      string                       `json:"location_id"`
			PaymentRequests []map[string]json.RawMessage `json:"payment_requests"`
		} `json:"invoice"`
	}
	if err := json.Unmarshal(fake.createPayloads[0], &create); err != nil {
		t.Fatal(err)
	}
	if create.Invoice.DeliveryMethod != "SHARE_MANUALLY" || create.Invoice.PrimaryRecipient.CustomerID != "ORIGINAL_CUSTOMER" || create.Invoice.LocationID != "ORIGINAL_LOC" {
		t.Fatalf("changed Square ownership/delivery: %+v", create)
	}
	if len(create.Invoice.PaymentRequests) != 1 {
		t.Fatalf("expected one balance payment request: %+v", create.Invoice.PaymentRequests)
	}
	var dueDate string
	if err := json.Unmarshal(create.Invoice.PaymentRequests[0]["due_date"], &dueDate); err != nil {
		t.Fatal(err)
	}
	if dueDate != beforeSend && dueDate != afterSend {
		t.Fatalf("invoice due date = %s, want 15 days from creation (%s or %s)", dueDate, beforeSend, afterSend)
	}
	if !strings.Contains(links[0], dueDate) {
		t.Fatal("invoice email omitted the Square payment due date")
	}
	for _, request := range create.Invoice.PaymentRequests {
		if len(request["reminders"]) != 0 || len(request["automatic_payment_source"]) != 0 {
			t.Fatal("automatic email/payment enabled")
		}
	}
	company.Set("billingEmails", []string{"changed@example.com"})
	if err := app.Save(company); err != nil {
		t.Fatal(err)
	}
	fake.url = "https://square.example/invoice/fresh"
	failSecond = false
	result = postInvoice(t, app, staff, order, sq, []string{"first@example.com", "second@example.com"})
	if string(result["notification_sent"]) != "true" {
		t.Fatalf("retry failed: %s", result["delivery"])
	}
	postInvoice(t, app, staff, order, sq, []string{"first@example.com", "second@example.com"})
	if !reflect.DeepEqual(submitted, []string{"first@example.com", "second@example.com"}) || fake.creates != 1 || fake.publishes != 1 || fake.gets != 1 {
		t.Fatalf("duplicated delivery: %v, Square=%+v", submitted, fake)
	}
	if !strings.Contains(links[1], fake.url) {
		t.Fatal("retry used stale payment URL")
	}
	order, _ = app.FindRecordById("orders", order.Id)
	if order.GetString("invoiceEmailSentAt") == "" || order.GetString("invoiceEmailError") != "" {
		t.Fatal("completion timestamp/error not recovered")
	}
}

func TestInvoicePublicationRetryResumesSavedDraft(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint("publication_committed=", committed), func(t *testing.T) {
			app, customer, _, _, staff := newCatalogAccessTestApp(t)
			order := invoiceTestOrder(t, app, customer, nil)
			sq, fake := newInvoiceSquare(t)
			fake.publishFailures, fake.publishCommittedFailure = 1, committed
			mails := 0
			app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error { mails++; return nil })
			postInvoice(t, app, staff, order, sq, []string{customer.Email()})
			order, _ = app.FindRecordById("orders", order.Id)
			if order.GetString("squareInvoiceId") != "INV1" || order.GetString("status") != "confirmed" || mails != 0 {
				t.Fatal("failed publication lost draft or sent email")
			}
			postInvoice(t, app, staff, order, sq, []string{customer.Email()})
			wantPublishes := 2
			if committed {
				wantPublishes = 1
			}
			if fake.creates != 1 || fake.publishes != wantPublishes || mails != 1 {
				t.Fatalf("resume counts: creates=%d publishes=%d mails=%d", fake.creates, fake.publishes, mails)
			}
		})
	}
}

func TestInvoiceRejectsStalePreviewAndPreservesLegacy(t *testing.T) {
	app, customer, _, _, staff := newCatalogAccessTestApp(t)
	company := invoiceTestCompany(t, app, "Account", []string{"new@example.com"})
	order := invoiceTestOrder(t, app, customer, company)
	response, err := invokeCatalogAccessHandler(t, app, staff, "POST", "/invoices", map[string]any{"order_id": order.Id, "recipients": []string{"old@example.com"}}, nil, handleSendInvoice(nil, "LOC"))
	if err != nil || response.Code != 409 {
		t.Fatalf("stale preview: %d, %v", response.Code, err)
	}
	order, _ = app.FindRecordById("orders", order.Id)
	if order.GetString("invoiceDraftRequest") != "" && order.GetString("invoiceDraftRequest") != "null" {
		t.Fatal("stale preview started invoice")
	}
	order.Set("squareInvoiceId", "LEGACY")
	if err := app.Save(order); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(order)
	delivery, err := getInvoiceDelivery(app, order)
	if err != nil || delivery.Status != "legacy" {
		t.Fatalf("legacy preview: %+v, %v", delivery, err)
	}
	response, err = invokeCatalogAccessHandler(t, app, staff, "POST", "/invoices", map[string]any{"order_id": order.Id, "recipients": []string{"new@example.com"}}, nil, handleSendInvoice(nil, "LOC"))
	if err != nil || response.Code != 409 {
		t.Fatalf("legacy send: %d, %v", response.Code, err)
	}
	order, _ = app.FindRecordById("orders", order.Id)
	after, _ := json.Marshal(order)
	if !bytes.Equal(before, after) {
		t.Fatal("legacy invoice was modified")
	}
}

func TestInvoiceRetryDoesNotEmailUnpayableInvoice(t *testing.T) {
	for _, status := range []string{"PAID", "CANCELED", "PAYMENT_PENDING", "PARTIALLY_PAID"} {
		t.Run(status, func(t *testing.T) {
			app, customer, _, _, staff := newCatalogAccessTestApp(t)
			order := invoiceTestOrder(t, app, customer, nil)
			sq, fake := newInvoiceSquare(t)
			fake.publishFailures = 1
			mails := 0
			app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error { mails++; return nil })
			postInvoice(t, app, staff, order, sq, []string{customer.Email()})
			fake.status = status
			result := postInvoice(t, app, staff, order, sq, []string{customer.Email()})
			if mails != 0 || string(result["notification_sent"]) != "false" || !strings.Contains(string(result["delivery"]), status) {
				t.Fatalf("unsafe invoice email: %s", result["delivery"])
			}
		})
	}
}

func TestInvoiceConcurrentSubmissionsSendOnce(t *testing.T) {
	app, customer, _, _, staff := newCatalogAccessTestApp(t)
	order := invoiceTestOrder(t, app, customer, nil)
	sq, fake := newInvoiceSquare(t)
	mails := 0
	app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error { mails++; return nil })
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); postInvoice(t, app, staff, order, sq, []string{customer.Email()}) }()
	}
	wg.Wait()
	if mails != 1 || fake.creates != 1 || fake.publishes != 1 {
		t.Fatalf("duplicate submission: mails=%d creates=%d publishes=%d", mails, fake.creates, fake.publishes)
	}
}

func TestInvoiceAmbiguousCreateKeepsIdempotentPayloadAndSnapshot(t *testing.T) {
	app, customer, _, _, staff := newCatalogAccessTestApp(t)
	company := invoiceTestCompany(t, app, "Account", []string{"billing@example.com"})
	order := invoiceTestOrder(t, app, customer, company)
	sq, fake := newInvoiceSquare(t)
	fake.createFailures = 1
	mails := 0
	app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error { mails++; return nil })
	result := postInvoice(t, app, staff, order, sq, []string{"billing@example.com"})
	if string(result["notification_sent"]) != "false" || mails != 0 {
		t.Fatal("ambiguous create reported delivery")
	}
	order, _ = app.FindRecordById("orders", order.Id)
	if order.GetString("squareInvoiceId") != "" {
		t.Fatal("invented draft ID")
	}
	company.Set("billingEmails", []string{"changed@example.com"})
	customer.Set("squareCustomerId", "CHANGED_CUSTOMER")
	if err := app.Save(company); err != nil {
		t.Fatal(err)
	}
	if err := app.Save(customer); err != nil {
		t.Fatal(err)
	}
	postInvoice(t, app, staff, order, sq, []string{"billing@example.com"})
	if fake.creates != 2 || !bytes.Equal(fake.createPayloads[0], fake.createPayloads[1]) || mails != 1 {
		t.Fatalf("create retry changed its idempotent request: payloads=%q, mails=%d", fake.createPayloads, mails)
	}
}

func TestInvoiceUnusableURLLeavesPublishedInvoiceRetryable(t *testing.T) {
	for _, paymentURL := range []string{"", "http://square.example/pay", "javascript:alert(1)", "https://user:password@square.example/pay"} {
		t.Run(paymentURL, func(t *testing.T) {
			app, customer, _, _, staff := newCatalogAccessTestApp(t)
			order := invoiceTestOrder(t, app, customer, nil)
			sq, fake := newInvoiceSquare(t)
			fake.url = paymentURL
			mails := 0
			app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error { mails++; return nil })
			result := postInvoice(t, app, staff, order, sq, []string{customer.Email()})
			order, _ = app.FindRecordById("orders", order.Id)
			if order.GetString("status") != "invoiced" || mails != 0 || string(result["notification_sent"]) != "false" {
				t.Fatal("unusable URL sent or publication lost")
			}
			fake.url = "https://square.example/usable"
			postInvoice(t, app, staff, order, sq, []string{customer.Email()})
			if mails != 1 || fake.creates != 1 || fake.publishes != 1 {
				t.Fatal("URL retry recreated or republished invoice")
			}
		})
	}
}
