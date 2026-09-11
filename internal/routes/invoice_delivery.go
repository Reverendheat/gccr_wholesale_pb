package routes

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
	"github.com/reverendheat/gccr_invoice/internal/billing"
	"github.com/reverendheat/gccr_invoice/internal/orders"
	"github.com/reverendheat/gccr_invoice/internal/orders/fsm"
	"github.com/reverendheat/gccr_invoice/internal/square"
	squaresdk "github.com/square/square-go-sdk/v3"
)

// This single-process portal serializes invoice submissions, including SMTP and
// durable progress saves. No database transaction is held over network calls.
var invoiceDeliveryMu sync.Mutex

type invoiceDelivery struct {
	Recipients        []string `json:"recipients"`
	SentRecipients    []string `json:"sent_recipients"`
	PendingRecipients []string `json:"pending_recipients"`
	Status            string   `json:"status"`
	Error             string   `json:"error"`
}

type invoiceDraftRequest struct {
	CustomerID string `json:"customer_id"`
	LocationID string `json:"location_id"`
	DueDate    string `json:"due_date"`
}

func recordEmailList(record *core.Record, field string) ([]string, error) {
	return billing.NormalizeEmails([]byte(record.GetString(field)))
}

func selectedInvoiceRecipients(app core.App, order *core.Record) ([]string, error) {
	// History belongs to the order's account, not the customer's current account.
	if companyID := order.GetString("company"); companyID != "" {
		company, err := app.FindRecordById("companies", companyID)
		if err != nil {
			return nil, fmt.Errorf("order company could not be found: %w", err)
		}
		emails, err := recordEmailList(company, "billingEmails")
		if err != nil {
			return nil, fmt.Errorf("order company billing emails: %w", err)
		}
		if len(emails) > 0 {
			return emails, nil
		}
	}
	customer, err := app.FindRecordById("customers", order.GetString("customer"))
	if err != nil {
		return nil, fmt.Errorf("order customer could not be found: %w", err)
	}
	raw, _ := json.Marshal([]string{customer.Email()})
	emails, err := billing.NormalizeEmails(raw)
	if err != nil {
		return nil, fmt.Errorf("order customer account email: %w", err)
	}
	return emails, nil
}

func getInvoiceDelivery(app core.App, order *core.Record) (invoiceDelivery, error) {
	delivery := invoiceDelivery{Recipients: []string{}, SentRecipients: []string{}, PendingRecipients: []string{}, Status: "not_created", Error: order.GetString("invoiceEmailError")}
	recipients, err := recordEmailList(order, "invoiceRecipients")
	if err != nil {
		return delivery, fmt.Errorf("invalid invoice recipient snapshot: %w", err)
	}
	if len(recipients) == 0 && order.GetString("squareInvoiceId") != "" {
		delivery.Status = "legacy"
		return delivery, nil
	}
	if len(recipients) == 0 {
		recipients, err = selectedInvoiceRecipients(app, order)
		if err != nil {
			return delivery, err
		}
	} else {
		delivery.Status = "pending"
	}
	sent, err := recordEmailList(order, "invoiceSentRecipients")
	if err != nil {
		return delivery, fmt.Errorf("invalid invoice sent recipients: %w", err)
	}
	delivery.Recipients, delivery.SentRecipients = recipients, sent
	for _, recipient := range recipients {
		if !slices.Contains(sent, recipient) {
			delivery.PendingRecipients = append(delivery.PendingRecipients, recipient)
		}
	}
	if delivery.Status == "pending" && len(delivery.PendingRecipients) == 0 && order.GetString("invoiceEmailSentAt") != "" && delivery.Error == "" {
		delivery.Status = "sent"
	}
	return delivery, nil
}

func handleInvoiceDelivery() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "users" {
			return e.ForbiddenError("Only staff can review invoice delivery", nil)
		}
		order, err := e.App.FindRecordById("orders", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Order not found", err)
		}
		delivery, err := getInvoiceDelivery(e.App, order)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}
		return e.JSON(http.StatusOK, delivery)
	}
}

func handleUpdateCompanyBilling() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "users" {
			return e.ForbiddenError("Only staff can configure billing emails", nil)
		}
		var body struct {
			BillingEmails json.RawMessage `json:"billing_emails"`
		}
		if err := e.BindBody(&body); err != nil || len(body.BillingEmails) == 0 {
			return e.BadRequestError("billing_emails is required", err)
		}
		emails, err := billing.NormalizeEmails(body.BillingEmails)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}
		company, err := e.App.FindRecordById("companies", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Company not found", err)
		}
		company.Set("billingEmails", emails)
		if err := e.App.Save(company); err != nil {
			return e.InternalServerError("Could not save billing emails", err)
		}
		return e.JSON(http.StatusOK, map[string]any{"company": company})
	}
}

type sendInvoiceBody struct {
	OrderID    string          `json:"order_id"`
	Recipients json.RawMessage `json:"recipients"`
}

func handleSendInvoice(sq *square.Client, locationID string) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "users" {
			return e.ForbiddenError("Only staff can send invoices", nil)
		}
		var body sendInvoiceBody
		if err := e.BindBody(&body); err != nil || body.OrderID == "" {
			return e.BadRequestError("order_id and reviewed recipients are required", err)
		}
		expected, err := billing.NormalizeEmails(body.Recipients)
		if err != nil || len(expected) == 0 {
			return e.BadRequestError("A valid reviewed recipient list is required", err)
		}
		invoiceDeliveryMu.Lock()
		defer invoiceDeliveryMu.Unlock()
		order, err := e.App.FindRecordById("orders", body.OrderID)
		if err != nil {
			return e.NotFoundError("Order not found", err)
		}
		delivery, err := getInvoiceDelivery(e.App, order)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}
		if delivery.Status == "legacy" {
			return e.JSON(http.StatusConflict, map[string]any{"message": "This existing invoice uses legacy Square delivery and cannot be sent through the portal", "delivery": delivery})
		}
		if len(expected) != len(delivery.Recipients) || !allRecipientsMatch(expected, delivery.Recipients) {
			return e.JSON(http.StatusConflict, map[string]any{"message": "Billing recipients changed; review the current list before sending", "delivery": delivery})
		}
		if delivery.Status == "sent" {
			return invoiceDeliveryResponse(e, order)
		}
		if order.GetString("status") != string(fsm.StatusInvoiced) {
			if _, err := fsm.Apply(fsm.Status(order.GetString("status")), fsm.EventStaffSendInvoice); err != nil {
				return e.BadRequestError(err.Error(), err)
			}
		}
		var draft invoiceDraftRequest
		if delivery.Status == "not_created" {
			customer, err := e.App.FindRecordById("customers", order.GetString("customer"))
			if err != nil || customer.GetString("squareCustomerId") == "" {
				return e.BadRequestError("Order customer has no valid Square customer ID", err)
			}
			draft = invoiceDraftRequest{CustomerID: customer.GetString("squareCustomerId"), LocationID: locationID, DueDate: time.Now().AddDate(0, 0, 15).Format("2006-01-02")}
			if squareOrderID := order.GetString("squareOrderId"); squareOrderID != "" {
				squareOrder, err := sq.GetOrder(e.Request.Context(), squareOrderID)
				if err != nil {
					return e.BadRequestError("Could not verify the original Square order customer", err)
				}
				draft.CustomerID, draft.LocationID = *squareOrder.CustomerID, squareOrder.LocationID
			}
			if draft.LocationID == "" {
				return e.BadRequestError("Square location is required", nil)
			}
			order.Set("invoiceRecipients", delivery.Recipients)
			order.Set("invoiceSentRecipients", []string{})
			order.Set("invoiceDraftRequest", draft)
			if err := e.App.Save(order); err != nil {
				return e.InternalServerError("Could not save invoice delivery snapshot", err)
			}
		} else if err := order.UnmarshalJSONField("invoiceDraftRequest", &draft); err != nil || draft.CustomerID == "" || draft.LocationID == "" || draft.DueDate == "" {
			return e.BadRequestError("Invoice attempt is missing its saved Square request; staff must reconcile it before retrying", err)
		}
		if err := deliverInvoice(e, sq, order, draft); err != nil {
			latest, findErr := e.App.FindRecordById("orders", order.Id)
			if findErr != nil {
				return e.InternalServerError("Could not refresh failed invoice delivery", findErr)
			}
			order.Set("status", latest.GetString("status"))
			order.Set("invoiceEmailError", err.Error())
			if saveErr := e.App.Save(order); saveErr != nil {
				return e.InternalServerError("Could not persist invoice delivery progress; reconcile delivery before retrying", saveErr)
			}
		}
		return invoiceDeliveryResponse(e, order)
	}
}

func allRecipientsMatch(expected, actual []string) bool {
	for _, email := range expected {
		if !slices.Contains(actual, email) {
			return false
		}
	}
	return true
}

func invoiceDeliveryResponse(e *core.RequestEvent, order *core.Record) error {
	delivery, err := getInvoiceDelivery(e.App, order)
	if err != nil {
		return e.InternalServerError("Could not read invoice delivery", err)
	}
	_ = e.App.ExpandRecord(order, []string{"customer"}, nil)
	return e.JSON(http.StatusOK, map[string]any{
		"order": order, "invoice_url": order.GetString("squareInvoiceUrl"),
		"notification_sent": delivery.Status == "sent", "delivery": delivery,
	})
}

func deliverInvoice(e *core.RequestEvent, sq *square.Client, order *core.Record, draft invoiceDraftRequest) error {
	ctx := e.Request.Context()
	var invoice *squaresdk.Invoice
	var err error
	if invoiceID := order.GetString("squareInvoiceId"); invoiceID != "" {
		invoice, err = sq.GetInvoice(ctx, invoiceID)
	} else {
		var squareOrderID string
		squareOrderID, err = orders.SubmitToSquare(ctx, e.App, sq, draft.LocationID, draft.CustomerID, order)
		if err == nil {
			invoice, err = sq.CreateInvoice(ctx, squareOrderID, draft.LocationID, draft.CustomerID, draft.DueDate, order.Id)
		}
		if err == nil {
			order.Set("squareInvoiceId", *invoice.ID)
			err = e.App.Save(order)
		}
	}
	if err != nil {
		return err
	}
	if invoice.DeliveryMethod == nil || *invoice.DeliveryMethod != squaresdk.InvoiceDeliveryMethodShareManually || invoice.OrderID == nil || *invoice.OrderID != order.GetString("squareOrderId") {
		return fmt.Errorf("Square invoice no longer matches this manual-delivery order; staff must reconcile it")
	}
	if *invoice.Status == squaresdk.InvoiceStatusDraft {
		invoice, err = sq.PublishInvoice(ctx, invoice, order.Id)
		if err != nil {
			return err
		}
	}
	if *invoice.Status != squaresdk.InvoiceStatusUnpaid {
		return fmt.Errorf("Square invoice is %s; no payment request was emailed", *invoice.Status)
	}
	// Re-read status after network calls so a webhook's terminal transition is
	// never overwritten by the original request's stale status.
	latest, err := e.App.FindRecordById("orders", order.Id)
	if err != nil {
		return err
	}
	order.Set("status", latest.GetString("status"))
	if latest.GetString("status") != string(fsm.StatusInvoiced) {
		next, err := fsm.Apply(fsm.Status(latest.GetString("status")), fsm.EventStaffSendInvoice)
		if err != nil {
			order.Set("status", latest.GetString("status"))
			return err
		}
		order.Set("status", string(next))
	}
	if err := e.App.Save(order); err != nil {
		return err
	}
	if invoice.PublicURL == nil {
		return fmt.Errorf("Square invoice has no payment URL; retry after Square makes it available")
	}
	paymentURL, err := url.Parse(*invoice.PublicURL)
	if err != nil || paymentURL.Scheme != "https" || paymentURL.Hostname() == "" || paymentURL.User != nil {
		return fmt.Errorf("Square invoice has an unusable payment URL")
	}
	order.Set("squareInvoiceUrl", paymentURL.String())
	if err := e.App.Save(order); err != nil {
		return err
	}
	delivery, err := getInvoiceDelivery(e.App, order)
	if err != nil {
		return err
	}
	settings := e.App.Settings()
	for _, recipient := range delivery.PendingRecipients {
		message := &mailer.Message{
			From:    mail.Address{Name: settings.Meta.SenderName, Address: settings.Meta.SenderAddress},
			To:      []mail.Address{{Address: recipient}},
			Subject: "Your GCCR Wholesale invoice",
			HTML:    fmt.Sprintf(`<p>Your GCCR Wholesale invoice for order <strong>%s</strong> is ready.</p><p><a href="%s">View and pay your invoice</a></p><p>Payment due %s.</p>`, html.EscapeString(order.Id), html.EscapeString(paymentURL.String()), html.EscapeString(draft.DueDate)),
		}
		if err := e.App.NewMailClient().Send(message); err != nil {
			return fmt.Errorf("Invoice email to %s failed: %w", recipient, err)
		}
		// SMTP acceptance is not proof of inbox delivery. Saving each known
		// success avoids resending it on ordinary retries; a crash between SMTP
		// acceptance and this save remains an unavoidable ambiguity.
		delivery.SentRecipients = append(delivery.SentRecipients, recipient)
		order.Set("invoiceSentRecipients", delivery.SentRecipients)
		if err := e.App.Save(order); err != nil {
			return fmt.Errorf("SMTP accepted email to %s but saving progress failed; reconcile before retrying: %w", recipient, err)
		}
	}
	order.Set("invoiceEmailError", "")
	order.Set("invoiceEmailSentAt", time.Now().UTC())
	return e.App.Save(order)
}
