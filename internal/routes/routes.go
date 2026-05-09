// Package routes registers custom HTTP routes on the PocketBase server.
package routes

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
	"github.com/reverendheat/gccr_invoice/internal/orders"
	"github.com/reverendheat/gccr_invoice/internal/square"
	squaresdk "github.com/square/square-go-sdk/v3"
)

// Register binds all custom application routes onto the serve event.
func Register(se *core.ServeEvent, sq *square.Client, locationID string) {
	g := se.Router.Group("/api/wholesale")
	g.Bind(apis.RequireAuth())

	// GET /api/wholesale/catalog — returns active wholesale items from Square
	g.GET("/catalog", handleCatalog(sq))

	// POST /api/wholesale/orders — customer submits a one-time order
	g.POST("/orders", handleCreateOrder(sq, locationID))

	// GET /api/wholesale/orders — list orders (customers see own; staff see all)
	g.GET("/orders", handleListOrders())

	// POST /api/wholesale/scheduled-orders — customer creates a recurring order
	g.POST("/scheduled-orders", handleCreateScheduledOrder(sq, locationID))

	// GET /api/wholesale/scheduled-orders — list active scheduled orders
	g.GET("/scheduled-orders", handleListScheduledOrders())

	// DELETE /api/wholesale/scheduled-orders/{id} — customer cancels a scheduled order
	g.DELETE("/scheduled-orders/{id}", handleCancelScheduledOrder())

	// POST /api/wholesale/invoices — staff sends a Square invoice for an order
	g.POST("/invoices", handleSendInvoice(sq, locationID))

	// POST /api/wholesale/invite — staff invites a customer by email (looks up Square, creates account, emails welcome link)
	g.POST("/invite", handleInviteCustomer(sq))

	// Square webhook — must be outside the auth group; Square sends no auth token.
	registerWebhook(se, os.Getenv("SQUARE_WEBHOOK_SIGNATURE_KEY"), os.Getenv("SQUARE_WEBHOOK_URL"))
}

// --- handlers ---

func handleCatalog(sq *square.Client) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		items, err := sq.GetWholesaleCatalog(e.Request.Context())
		if err != nil {
			return e.InternalServerError("Could not fetch catalog", err)
		}
		return e.JSON(http.StatusOK, map[string]any{"items": items})
	}
}

// orderLineItemInput is the shape expected in order request bodies.
type orderLineItemInput struct {
	VariationID string `json:"variation_id"`
	Quantity    int    `json:"quantity"`
	Note        string `json:"note"`
}

type createOrderBody struct {
	LineItems []orderLineItemInput `json:"lineItems"`
	Notes     string               `json:"notes"`
}

func validateLineItems(items []orderLineItemInput) error {
	if len(items) == 0 {
		return fmt.Errorf("at least one line item is required")
	}
	for _, li := range items {
		if li.VariationID == "" {
			return fmt.Errorf("each line item must have a variation_id")
		}
		if li.Quantity <= 0 {
			return fmt.Errorf("quantity must be greater than zero")
		}
	}
	return nil
}

func toOrderLineItems(inputs []orderLineItemInput) []orders.LineItem {
	out := make([]orders.LineItem, len(inputs))
	for i, li := range inputs {
		out[i] = orders.LineItem{
			VariationID: li.VariationID,
			Quantity:    li.Quantity,
			Note:        li.Note,
		}
	}
	return out
}

func handleCreateOrder(sq *square.Client, locationID string) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "customers" {
			return e.ForbiddenError("Only customers can place orders", nil)
		}

		var body createOrderBody
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Invalid request body", err)
		}
		if err := validateLineItems(body.LineItems); err != nil {
			return e.BadRequestError(err.Error(), nil)
		}

		squareCustomerID := e.Auth.GetString("squareCustomerId")
		if squareCustomerID == "" {
			return e.ForbiddenError("Account is not linked to Square", nil)
		}

		pbOrder, err := orders.Create(
			e.Request.Context(), e.App, sq,
			locationID, e.Auth.Id, squareCustomerID,
			toOrderLineItems(body.LineItems), body.Notes,
			fmt.Sprintf("order-%s", e.Auth.Id+time.Now().UTC().Format("20060102150405")),
		)
		if err != nil {
			return e.InternalServerError("Could not create order", err)
		}

		return e.JSON(http.StatusOK, pbOrder)
	}
}

func handleListOrders() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return e.ForbiddenError("Authentication required", nil)
		}

		var records []*core.Record
		var err error

		switch e.Auth.Collection().Name {
		case "users":
			records, err = e.App.FindRecordsByFilter(
				"orders", "id != ''", "-id", 100, 0,
			)
		case "customers":
			records, err = e.App.FindRecordsByFilter(
				"orders",
				fmt.Sprintf("customer = '%s'", e.Auth.Id),
				"-id", 100, 0,
			)
		default:
			return e.ForbiddenError("Unknown account type", nil)
		}

		if err != nil {
			return e.InternalServerError("Could not fetch orders", err)
		}

		// Expand the customer relation so the frontend can show customer names.
		for _, rec := range records {
			_ = e.App.ExpandRecord(rec, []string{"customer"}, nil)
		}

		return e.JSON(http.StatusOK, map[string]any{"orders": records})
	}
}

// --- scheduled orders ---

var validFrequencies = map[string]bool{
	"weekly":    true,
	"biweekly":  true,
	"monthly":   true,
	"quarterly": true,
}

type createScheduledOrderBody struct {
	LineItems []orderLineItemInput `json:"lineItems"`
	Notes     string               `json:"notes"`
	Frequency string               `json:"frequency"`
}

func handleCreateScheduledOrder(sq *square.Client, locationID string) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "customers" {
			return e.ForbiddenError("Only customers can create scheduled orders", nil)
		}

		var body createScheduledOrderBody
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Invalid request body", err)
		}
		if err := validateLineItems(body.LineItems); err != nil {
			return e.BadRequestError(err.Error(), nil)
		}
		if !validFrequencies[body.Frequency] {
			return e.BadRequestError("frequency must be weekly, biweekly, monthly, or quarterly", nil)
		}

		squareCustomerID := e.Auth.GetString("squareCustomerId")
		if squareCustomerID == "" {
			return e.ForbiddenError("Account is not linked to Square", nil)
		}

		lineItems := toOrderLineItems(body.LineItems)

		// Place the first order immediately so the customer sees instant confirmation.
		firstOrder, err := orders.Create(
			e.Request.Context(), e.App, sq,
			locationID, e.Auth.Id, squareCustomerID,
			lineItems, body.Notes,
			fmt.Sprintf("sched-first-%s-%s", e.Auth.Id, time.Now().UTC().Format("20060102150405")),
		)
		if err != nil {
			return e.InternalServerError("Could not create initial order", err)
		}

		// Persist the schedule so the cron job can fire subsequent orders.
		scheduledCol, err := e.App.FindCollectionByNameOrId("scheduledOrders")
		if err != nil {
			return e.InternalServerError("Scheduled orders collection not found", err)
		}

		lineItemsSnapshot := make([]map[string]any, len(body.LineItems))
		for i, li := range body.LineItems {
			lineItemsSnapshot[i] = map[string]any{
				"variation_id": li.VariationID,
				"quantity":     li.Quantity,
				"note":         li.Note,
			}
		}

		nextRunAt := advanceBy(time.Now().UTC(), body.Frequency)

		sr := core.NewRecord(scheduledCol)
		sr.Set("customer", e.Auth.Id)
		sr.Set("frequency", body.Frequency)
		sr.Set("lineItems", lineItemsSnapshot)
		sr.Set("notes", body.Notes)
		sr.Set("next_run_at", nextRunAt.Format("2006-01-02 15:04:05.000Z"))
		sr.Set("active", true)

		if err := e.App.Save(sr); err != nil {
			return e.InternalServerError("Could not save scheduled order", err)
		}

		// Re-fetch so the response includes server-populated timestamps.
		sr, err = e.App.FindRecordById("scheduledOrders", sr.Id)
		if err != nil {
			return e.InternalServerError("Could not fetch saved scheduled order", err)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"order":          firstOrder,
			"scheduledOrder": sr,
		})
	}
}

func handleListScheduledOrders() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return e.ForbiddenError("Authentication required", nil)
		}

		var records []*core.Record
		var err error

		switch e.Auth.Collection().Name {
		case "users":
			records, err = e.App.FindRecordsByFilter(
				"scheduledOrders", "id != ''", "-id", 200, 0,
			)
		case "customers":
			records, err = e.App.FindRecordsByFilter(
				"scheduledOrders",
				fmt.Sprintf("customer = '%s' && active = true", e.Auth.Id),
				"-id", 200, 0,
			)
		default:
			return e.ForbiddenError("Unknown account type", nil)
		}

		if err != nil {
			return e.InternalServerError("Could not fetch scheduled orders", err)
		}

		return e.JSON(http.StatusOK, map[string]any{"scheduledOrders": records})
	}
}

func handleCancelScheduledOrder() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return e.ForbiddenError("Authentication required", nil)
		}

		id := e.Request.PathValue("id")
		sr, err := e.App.FindRecordById("scheduledOrders", id)
		if err != nil {
			return e.NotFoundError("Scheduled order not found", err)
		}

		// Customers may only cancel their own; staff may cancel any.
		if e.Auth.Collection().Name == "customers" && sr.GetString("customer") != e.Auth.Id {
			return e.ForbiddenError("You can only cancel your own scheduled orders", nil)
		}

		sr.Set("active", false)
		if err := e.App.Save(sr); err != nil {
			return e.InternalServerError("Could not cancel scheduled order", err)
		}

		return e.JSON(http.StatusOK, map[string]any{"cancelled": true})
	}
}

type sendInvoiceBody struct {
	OrderID string `json:"order_id"`
}

func handleSendInvoice(sq *square.Client, locationID string) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "users" {
			return e.ForbiddenError("Only staff can send invoices", nil)
		}

		var body sendInvoiceBody
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Invalid request body", err)
		}
		if body.OrderID == "" {
			return e.BadRequestError("order_id is required", nil)
		}

		order, err := e.App.FindRecordById("orders", body.OrderID)
		if err != nil {
			return e.NotFoundError("Order not found", err)
		}

		squareOrderID := order.GetString("squareOrderId")
		if squareOrderID == "" {
			return e.BadRequestError("Order has no Square order ID", nil)
		}
		if order.GetString("squareInvoiceId") != "" {
			return e.BadRequestError("Invoice already sent for this order", nil)
		}

		customerRecord, err := e.App.FindRecordById("customers", order.GetString("customer"))
		if err != nil {
			return e.InternalServerError("Could not find customer", err)
		}
		squareCustomerID := customerRecord.GetString("squareCustomerId")
		if squareCustomerID == "" {
			return e.BadRequestError("Customer has no Square customer ID", nil)
		}

		// Default due date: 30 days from today.
		dueDate := time.Now().AddDate(0, 0, 30).Format("2006-01-02")

		invoice, err := sq.CreateAndPublishInvoice(
			e.Request.Context(),
			squareOrderID, locationID, squareCustomerID,
			dueDate, order.Id,
		)
		if err != nil {
			return e.InternalServerError("Could not create Square invoice", err)
		}

		order.Set("squareInvoiceId", *invoice.ID)
		order.Set("status", "invoiced")
		if err := e.App.Save(order); err != nil {
			return e.InternalServerError("Could not update order", err)
		}

		// Re-fetch so response includes server-populated timestamps and expand.
		order, err = e.App.FindRecordById("orders", order.Id)
		if err != nil {
			return e.InternalServerError("Could not refresh order", err)
		}
		_ = e.App.ExpandRecord(order, []string{"customer"}, nil)

		publicURL := ""
		if invoice.PublicURL != nil {
			publicURL = *invoice.PublicURL
		}

		return e.JSON(http.StatusOK, map[string]any{
			"order":       order,
			"invoice_url": publicURL,
		})
	}
}

// --- invite ---

type inviteCustomerBody struct {
	Email string `json:"email"`
}

func handleInviteCustomer(sq *square.Client) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "users" {
			return e.ForbiddenError("Only staff can invite customers", nil)
		}

		var body inviteCustomerBody
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Invalid request body", err)
		}
		if body.Email == "" {
			return e.BadRequestError("email is required", nil)
		}

		// Look up the customer in Square by email.
		squareCustomer, err := sq.SearchCustomerByEmail(e.Request.Context(), body.Email)
		if err != nil {
			return e.InternalServerError("Could not search Square for customer", err)
		}
		if squareCustomer == nil {
			return e.BadRequestError("This email isn't registered in Square. Please add them as a customer in Square first, then invite them here.", nil)
		}

		// Check if a PocketBase customer account already exists for this email.
		existing, _ := e.App.FindAuthRecordByEmail("customers", body.Email)
		if existing != nil {
			return e.BadRequestError("A customer account already exists for this email", nil)
		}

		// Create the PocketBase customer record.
		col, err := e.App.FindCollectionByNameOrId("customers")
		if err != nil {
			return e.InternalServerError("customers collection not found", err)
		}

		givenName := ""
		if squareCustomer.GivenName != nil {
			givenName = *squareCustomer.GivenName
		}
		familyName := ""
		if squareCustomer.FamilyName != nil {
			familyName = *squareCustomer.FamilyName
		}
		name := givenName + " " + familyName
		if name == " " {
			name = body.Email
		}

		phoneNumber := ""
		if squareCustomer.PhoneNumber != nil {
			phoneNumber = *squareCustomer.PhoneNumber
		}
		squareID := ""
		if squareCustomer.ID != nil {
			squareID = *squareCustomer.ID
		}
		companyID, companyName, err := findOrCreateCompanyForSquareCustomer(e.App, squareCustomer)
		if err != nil {
			return e.InternalServerError("Could not associate customer company", err)
		}

		record := core.NewRecord(col)
		record.SetEmail(body.Email)
		record.Set("name", name)
		record.Set("phone", phoneNumber)
		record.Set("squareCustomerId", squareID)
		if companyID != "" {
			record.Set("company", companyID)
		}
		// Set a random password even though customer password auth is disabled.
		record.SetPassword(core.GenerateDefaultRandomId() + core.GenerateDefaultRandomId())

		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("Could not create customer account", err)
		}

		if err := sendCustomerWelcomeEmail(e.App, record); err != nil {
			// Account was created; log but don't fail.
			e.App.Logger().Error("invite: failed to send welcome email",
				"customer_id", record.Id, "email", body.Email, "error", err)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"id":          record.Id,
			"email":       body.Email,
			"name":        name,
			"company":     companyID,
			"companyName": companyName,
		})
	}
}

func findOrCreateCompanyForSquareCustomer(app core.App, customer *squaresdk.Customer) (string, string, error) {
	companyName := ""
	if customer != nil && customer.CompanyName != nil {
		companyName = strings.TrimSpace(*customer.CompanyName)
	}
	if companyName == "" {
		return "", "", nil
	}

	existing, err := app.FindFirstRecordByFilter(
		"companies",
		"name={:name}",
		dbx.Params{"name": companyName},
	)
	if err == nil {
		if err := fillCompanyDetails(app, existing, customer); err != nil {
			return "", "", err
		}
		return existing.Id, companyName, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}

	collection, err := app.FindCollectionByNameOrId("companies")
	if err != nil {
		return "", "", err
	}

	record := core.NewRecord(collection)
	record.Set("name", companyName)
	setCompanyDetails(record, customer)

	if err := app.Save(record); err != nil {
		return "", "", err
	}

	return record.Id, companyName, nil
}

func fillCompanyDetails(app core.App, record *core.Record, customer *squaresdk.Customer) error {
	if customer == nil {
		return nil
	}

	changed := false

	setIfEmpty := func(field string, value string) {
		value = strings.TrimSpace(value)
		if value != "" && record.GetString(field) == "" {
			record.Set(field, value)
			changed = true
		}
	}

	setIfEmpty("address", formatSquareAddress(customer.Address))
	if customer.PhoneNumber != nil {
		setIfEmpty("phone", *customer.PhoneNumber)
	}
	if customer.EmailAddress != nil {
		setIfEmpty("email", *customer.EmailAddress)
	}

	if !changed {
		return nil
	}

	return app.Save(record)
}

func setCompanyDetails(record *core.Record, customer *squaresdk.Customer) {
	if customer == nil {
		return
	}

	record.Set("address", formatSquareAddress(customer.Address))
	if customer.PhoneNumber != nil {
		record.Set("phone", *customer.PhoneNumber)
	}
	if customer.EmailAddress != nil {
		record.Set("email", *customer.EmailAddress)
	}
}

func formatSquareAddress(address *squaresdk.Address) string {
	if address == nil {
		return ""
	}

	parts := []string{}
	add := func(value *string) {
		if value != nil && strings.TrimSpace(*value) != "" {
			parts = append(parts, strings.TrimSpace(*value))
		}
	}

	add(address.AddressLine1)
	add(address.AddressLine2)
	add(address.AddressLine3)
	add(address.Locality)
	add(address.AdministrativeDistrictLevel1)
	add(address.PostalCode)
	if address.Country != nil {
		parts = append(parts, fmt.Sprint(*address.Country))
	}

	return strings.Join(parts, ", ")
}

func sendCustomerWelcomeEmail(app core.App, record *core.Record) error {
	settings := app.Settings()
	appURL := strings.TrimRight(settings.Meta.AppURL, "/")
	customerName := strings.TrimSpace(record.GetString("name"))
	if customerName == "" {
		customerName = record.Email()
	}

	escapedName := html.EscapeString(customerName)
	escapedURL := html.EscapeString(appURL)

	message := &mailer.Message{
		From: mail.Address{
			Name:    settings.Meta.SenderName,
			Address: settings.Meta.SenderAddress,
		},
		To:      []mail.Address{{Address: record.Email()}},
		Subject: "Welcome to GCCR Wholesale",
		HTML: fmt.Sprintf(
			`<p>Hi %s,</p>
<p>Your GCCR Wholesale account has been created.</p>
<p><a href="%s">Open GCCR Wholesale</a>, choose Customer, and request a one-time sign-in code with this email address.</p>`,
			escapedName,
			escapedURL,
		),
	}

	return app.NewMailClient().Send(message)
}

// advanceBy returns the timestamp for the next run based on frequency.
func advanceBy(from time.Time, frequency string) time.Time {
	switch frequency {
	case "weekly":
		return from.AddDate(0, 0, 7)
	case "biweekly":
		return from.AddDate(0, 0, 14)
	case "monthly":
		return from.AddDate(0, 1, 0)
	case "quarterly":
		return from.AddDate(0, 3, 0)
	default:
		return from.AddDate(0, 0, 7)
	}
}
