// Package routes registers custom HTTP routes on the PocketBase server.
package routes

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
	"github.com/reverendheat/gccr_invoice/internal/delivery"
	"github.com/reverendheat/gccr_invoice/internal/orders"
	"github.com/reverendheat/gccr_invoice/internal/orders/fsm"
	"github.com/reverendheat/gccr_invoice/internal/square"
	squaresdk "github.com/square/square-go-sdk/v3"
)

// Register binds all custom application routes onto the serve event.
func Register(se *core.ServeEvent, sq *square.Client, locationID string, deliveryQuoter delivery.Quoter) {
	g := se.Router.Group("/api/wholesale")
	g.Bind(apis.RequireAuth())

	// GET /api/wholesale/catalog — returns active wholesale items from Square
	g.GET("/catalog", handleCatalog(sq))

	// POST /api/wholesale/fulfillment/quote — calculates authoritative delivery pricing
	g.POST("/fulfillment/quote", handleFulfillmentQuote(sq, locationID, deliveryQuoter))

	// POST /api/wholesale/orders — customer submits a one-time order
	g.POST("/orders", handleCreateOrder(sq, locationID, deliveryQuoter))

	// GET /api/wholesale/orders — customers see own/account records; staff see all
	g.GET("/orders", handleListOrders())

	// PATCH /api/wholesale/orders/{id} — customer edits their own pending order
	g.PATCH("/orders/{id}", handleUpdateOrder(sq, locationID, deliveryQuoter))

	// POST /api/wholesale/orders/{id}/events — staff applies a workflow event
	g.POST("/orders/{id}/events", handleOrderEvent())

	// POST /api/wholesale/scheduled-orders — customer creates a recurring order
	g.POST("/scheduled-orders", handleCreateScheduledOrder(sq, locationID, deliveryQuoter))

	// GET /api/wholesale/scheduled-orders — list active scheduled orders
	g.GET("/scheduled-orders", handleListScheduledOrders())

	// DELETE /api/wholesale/scheduled-orders/{id} — customer cancels a scheduled order
	g.DELETE("/scheduled-orders/{id}", handleCancelScheduledOrder())

	// POST /api/wholesale/invoices — staff sends a Square invoice for an order
	g.POST("/invoices", handleSendInvoice(sq, locationID))

	// Staff-controlled Square lookup and wholesale-account assignment.
	g.POST("/customers/preview", handlePreviewCustomer(sq))
	g.PATCH("/customers/{id}/account", handleAssignCustomerAccount())

	// POST /api/wholesale/invite — staff confirms account membership and invites a Square customer.
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
	LineItems   []orderLineItemInput `json:"lineItems"`
	Notes       string               `json:"notes"`
	Fulfillment orders.Fulfillment   `json:"fulfillment"`
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

func handleFulfillmentQuote(sq *square.Client, locationID string, deliveryQuoter delivery.Quoter) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "customers" {
			return e.ForbiddenError("Only customers can request fulfillment quotes", nil)
		}
		var body createOrderBody
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Invalid request body", err)
		}
		if err := validateLineItems(body.LineItems); err != nil {
			return e.BadRequestError(err.Error(), nil)
		}
		lineItems, err := orders.LockPrices(e.Request.Context(), sq, locationID, toOrderLineItems(body.LineItems))
		if err != nil {
			return e.BadRequestError("Could not price fulfillment quote", err)
		}
		fulfillment, subtotal, err := orders.QuoteFulfillment(e.Request.Context(), deliveryQuoter, body.Fulfillment, lineItems)
		if err != nil {
			return handleFulfillmentQuoteError(e, err)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"fulfillment":    fulfillment,
			"subtotal_cents": subtotal,
			"total_cents":    subtotal + fulfillment.FeeCents,
		})
	}
}

func handleFulfillmentQuoteError(e *core.RequestEvent, err error) error {
	if errors.Is(err, delivery.ErrOutsideDeliveryArea) || errors.Is(err, delivery.ErrAddressNotFound) {
		return e.BadRequestError(err.Error(), nil)
	}
	return e.InternalServerError("Could not calculate delivery quote", err)
}

// customerRecordFilter scopes records to those created by the customer or
// snapshotted to their wholesale account. Creator access remains after an
// account move, while account snapshots preserve historical visibility.
func customerRecordFilter(customerID, companyID string, activeOnly bool) string {
	filter := fmt.Sprintf("customer = '%s'", customerID)
	if companyID != "" {
		filter = fmt.Sprintf("(customer = '%s' || company = '%s')", customerID, companyID)
	}
	if activeOnly {
		filter += " && active = true"
	}
	return filter
}

func canCancelScheduledOrder(authCollection, authID, creatorID string) bool {
	switch authCollection {
	case "users":
		return true
	case "customers":
		return authID == creatorID
	default:
		return false
	}
}

func normalizeCompanyName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

func validateAccountSelection(companyID, newCompanyName string) error {
	hasExisting := strings.TrimSpace(companyID) != ""
	hasNew := strings.TrimSpace(newCompanyName) != ""
	if hasExisting == hasNew {
		return fmt.Errorf("select one existing account or enter one new account name")
	}
	return nil
}

func addSubmittedBy(app core.App, record *core.Record) {
	customer, err := app.FindRecordById("customers", record.GetString("customer"))
	if err != nil {
		return
	}
	record.Set("submittedBy", map[string]string{
		"id":   customer.Id,
		"name": customer.GetString("name"),
	})
	record.WithCustomData(true)
}

func handleCreateOrder(sq *square.Client, locationID string, deliveryQuoter delivery.Quoter) func(*core.RequestEvent) error {
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
		fulfillment, err := orders.NormalizeFulfillment(body.Fulfillment)
		if err != nil {
			return e.BadRequestError(err.Error(), nil)
		}

		if e.Auth.GetString("squareCustomerId") == "" {
			return e.ForbiddenError("Account is not linked to Square", nil)
		}

		lineItems, err := orders.LockPrices(e.Request.Context(), sq, locationID, toOrderLineItems(body.LineItems))
		if err != nil {
			return e.InternalServerError("Could not lock order pricing", err)
		}
		fulfillment, _, err = orders.QuoteFulfillment(e.Request.Context(), deliveryQuoter, fulfillment, lineItems)
		if err != nil {
			return handleFulfillmentQuoteError(e, err)
		}
		pbOrder, err := orders.Create(
			e.App, e.Auth.Id, e.Auth.GetString("company"),
			lineItems, fulfillment, body.Notes,
		)
		if err != nil {
			return e.InternalServerError("Could not create order", err)
		}

		return e.JSON(http.StatusOK, pbOrder)
	}
}

func customerCanEditOrder(customerID, ownerID, status string) bool {
	return customerID == ownerID && status == string(fsm.StatusPending)
}

func handleUpdateOrder(sq *square.Client, locationID string, deliveryQuoter delivery.Quoter) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "customers" {
			return e.ForbiddenError("Only customers can edit orders", nil)
		}

		var body createOrderBody
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Invalid request body", err)
		}
		if err := validateLineItems(body.LineItems); err != nil {
			return e.BadRequestError(err.Error(), nil)
		}
		fulfillment, err := orders.NormalizeFulfillment(body.Fulfillment)
		if err != nil {
			return e.BadRequestError(err.Error(), nil)
		}

		order, err := e.App.FindRecordById("orders", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Order not found", err)
		}
		if !customerCanEditOrder(e.Auth.Id, order.GetString("customer"), order.GetString("status")) {
			return e.ForbiddenError("Only your own pending orders can be edited", nil)
		}

		lineItems, err := orders.LockPrices(e.Request.Context(), sq, locationID, toOrderLineItems(body.LineItems))
		if err != nil {
			return e.BadRequestError("Could not update order pricing", err)
		}
		fulfillment, _, err = orders.QuoteFulfillment(e.Request.Context(), deliveryQuoter, fulfillment, lineItems)
		if err != nil {
			return handleFulfillmentQuoteError(e, err)
		}
		order, err = orders.UpdatePending(e.App, order, lineItems, fulfillment, body.Notes)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}
		addSubmittedBy(e.App, order)

		return e.JSON(http.StatusOK, order)
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
				customerRecordFilter(e.Auth.Id, e.Auth.GetString("company"), false),
				"-id", 100, 0,
			)
		default:
			return e.ForbiddenError("Unknown account type", nil)
		}

		if err != nil {
			return e.InternalServerError("Could not fetch orders", err)
		}

		for _, rec := range records {
			if e.Auth.Collection().Name == "users" {
				_ = e.App.ExpandRecord(rec, []string{"customer"}, nil)
			} else {
				// Customer responses expose submitter name only, never peer contact data.
				addSubmittedBy(e.App, rec)
				rec.Set("squareOrderId", "")
				rec.Set("squareInvoiceId", "")
			}
		}

		return e.JSON(http.StatusOK, map[string]any{"orders": records})
	}
}

type orderEventBody struct {
	Event string `json:"event"`
}

var staffOrderEvents = map[fsm.Event]bool{
	fsm.EventStaffConfirm:       true,
	fsm.EventStaffMarkDelivered: true,
	fsm.EventStaffCancel:        true,
}

func handleOrderEvent() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "users" {
			return e.ForbiddenError("Only staff can update order workflow", nil)
		}

		var body orderEventBody
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Invalid request body", err)
		}
		if body.Event == "" {
			return e.BadRequestError("event is required", nil)
		}
		event := fsm.Event(body.Event)
		if !staffOrderEvents[event] {
			return e.BadRequestError("event is not allowed for staff workflow updates", nil)
		}

		order, err := e.App.FindRecordById("orders", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Order not found", err)
		}

		next, err := fsm.Apply(fsm.Status(order.GetString("status")), event)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}

		order.Set("status", string(next))
		if err := e.App.Save(order); err != nil {
			return e.InternalServerError("Could not update order", err)
		}

		order, err = e.App.FindRecordById("orders", order.Id)
		if err != nil {
			return e.InternalServerError("Could not refresh order", err)
		}
		_ = e.App.ExpandRecord(order, []string{"customer"}, nil)

		return e.JSON(http.StatusOK, map[string]any{"order": order})
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
	LineItems   []orderLineItemInput `json:"lineItems"`
	Notes       string               `json:"notes"`
	Frequency   string               `json:"frequency"`
	Fulfillment orders.Fulfillment   `json:"fulfillment"`
}

func handleCreateScheduledOrder(sq *square.Client, locationID string, deliveryQuoter delivery.Quoter) func(*core.RequestEvent) error {
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
		fulfillment, err := orders.NormalizeFulfillment(body.Fulfillment)
		if err != nil {
			return e.BadRequestError(err.Error(), nil)
		}

		if e.Auth.GetString("squareCustomerId") == "" {
			return e.ForbiddenError("Account is not linked to Square", nil)
		}

		lineItems, err := orders.LockPrices(e.Request.Context(), sq, locationID, toOrderLineItems(body.LineItems))
		if err != nil {
			return e.InternalServerError("Could not lock initial order pricing", err)
		}
		scheduleFulfillment := fulfillment
		fulfillment, _, err = orders.QuoteFulfillment(e.Request.Context(), deliveryQuoter, fulfillment, lineItems)
		if err != nil {
			return handleFulfillmentQuoteError(e, err)
		}

		// Place the first local order immediately so the customer sees confirmation.
		firstOrder, err := orders.Create(
			e.App, e.Auth.Id, e.Auth.GetString("company"),
			lineItems, fulfillment, body.Notes,
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
		sr.Set("company", e.Auth.GetString("company"))
		sr.Set("frequency", body.Frequency)
		sr.Set("lineItems", lineItemsSnapshot)
		sr.Set("fulfillment", scheduleFulfillment)
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
				customerRecordFilter(e.Auth.Id, e.Auth.GetString("company"), true),
				"-id", 200, 0,
			)
		default:
			return e.ForbiddenError("Unknown account type", nil)
		}

		if err != nil {
			return e.InternalServerError("Could not fetch scheduled orders", err)
		}

		if e.Auth.Collection().Name == "customers" {
			for _, record := range records {
				addSubmittedBy(e.App, record)
			}
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

		// Account peers have shared visibility, but only the creator may cancel.
		if !canCancelScheduledOrder(e.Auth.Collection().Name, e.Auth.Id, sr.GetString("customer")) {
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

		if order.GetString("squareInvoiceId") != "" {
			return e.BadRequestError("Invoice already sent for this order", nil)
		}
		next, err := fsm.Apply(fsm.Status(order.GetString("status")), fsm.EventStaffSendInvoice)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}

		customerRecord, err := e.App.FindRecordById("customers", order.GetString("customer"))
		if err != nil {
			return e.InternalServerError("Could not find customer", err)
		}
		squareCustomerID := customerRecord.GetString("squareCustomerId")
		if squareCustomerID == "" {
			return e.BadRequestError("Customer has no Square customer ID", nil)
		}

		squareOrderID, err := orders.SubmitToSquare(
			e.Request.Context(), e.App, sq, locationID, squareCustomerID, order,
		)
		if err != nil {
			return e.InternalServerError("Could not create Square order", err)
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

		publicURL := ""
		if invoice.PublicURL != nil {
			publicURL = *invoice.PublicURL
		}

		order.Set("squareInvoiceId", *invoice.ID)
		order.Set("squareInvoiceUrl", publicURL)
		order.Set("status", string(next))
		if err := e.App.Save(order); err != nil {
			return e.InternalServerError("Could not update order", err)
		}

		// Re-fetch so response includes server-populated timestamps and expand.
		order, err = e.App.FindRecordById("orders", order.Id)
		if err != nil {
			return e.InternalServerError("Could not refresh order", err)
		}
		_ = e.App.ExpandRecord(order, []string{"customer"}, nil)

		return e.JSON(http.StatusOK, map[string]any{
			"order":       order,
			"invoice_url": publicURL,
		})
	}
}

// --- customer onboarding and account reconciliation ---

type accountSelectionBody struct {
	CompanyID      string `json:"company_id"`
	NewCompanyName string `json:"new_company_name"`
}

type previewCustomerBody struct {
	Email string `json:"email"`
}

type inviteCustomerBody struct {
	Email string `json:"email"`
	accountSelectionBody
}

type squareCustomerDetails struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	CompanyName string `json:"company_name"`
}

func customerDetails(customer *squaresdk.Customer, fallbackEmail string) squareCustomerDetails {
	details := squareCustomerDetails{Email: fallbackEmail}
	if customer.ID != nil {
		details.ID = *customer.ID
	}
	givenName := ""
	if customer.GivenName != nil {
		givenName = *customer.GivenName
	}
	familyName := ""
	if customer.FamilyName != nil {
		familyName = *customer.FamilyName
	}
	details.Name = strings.TrimSpace(givenName + " " + familyName)
	if details.Name == "" {
		details.Name = fallbackEmail
	}
	if customer.EmailAddress != nil {
		details.Email = *customer.EmailAddress
	}
	if customer.PhoneNumber != nil {
		details.Phone = *customer.PhoneNumber
	}
	if customer.CompanyName != nil {
		details.CompanyName = *customer.CompanyName
	}
	return details
}

func findSquareCustomer(sq *square.Client, e *core.RequestEvent, email string) (*squaresdk.Customer, error) {
	customer, err := sq.SearchCustomerByEmail(e.Request.Context(), strings.TrimSpace(email))
	if err != nil {
		return nil, e.InternalServerError("Could not search Square for customer", err)
	}
	if customer == nil {
		return nil, e.BadRequestError("This email isn't registered in Square. Please add them as a customer in Square first, then invite them here.", nil)
	}
	return customer, nil
}

func findSuggestedAccounts(app core.App, companyName string) ([]map[string]string, error) {
	suggested := make([]map[string]string, 0)
	if companyName == "" {
		return suggested, nil
	}

	companies, err := app.FindAllRecords("companies")
	if err != nil {
		return nil, err
	}
	for _, company := range companies {
		if normalizeCompanyName(company.GetString("name")) == normalizeCompanyName(companyName) {
			suggested = append(suggested, map[string]string{"id": company.Id, "name": company.GetString("name")})
		}
	}
	return suggested, nil
}

func handlePreviewCustomer(sq *square.Client) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "users" {
			return e.ForbiddenError("Only staff can preview customers", nil)
		}
		var body previewCustomerBody
		if err := e.BindBody(&body); err != nil || strings.TrimSpace(body.Email) == "" {
			return e.BadRequestError("email is required", err)
		}

		customer, err := findSquareCustomer(sq, e, body.Email)
		if err != nil {
			return err
		}
		details := customerDetails(customer, strings.TrimSpace(body.Email))

		suggested, err := findSuggestedAccounts(e.App, details.CompanyName)
		if err != nil {
			return e.InternalServerError("Could not search wholesale accounts", err)
		}

		return e.JSON(http.StatusOK, map[string]any{"customer": details, "suggested_accounts": suggested})
	}
}

func resolveCompany(app core.App, selection accountSelectionBody) (*core.Record, error) {
	if err := validateAccountSelection(selection.CompanyID, selection.NewCompanyName); err != nil {
		return nil, err
	}
	if selection.CompanyID != "" {
		return app.FindRecordById("companies", selection.CompanyID)
	}

	name := strings.TrimSpace(selection.NewCompanyName)
	companies, err := app.FindAllRecords("companies")
	if err != nil {
		return nil, err
	}
	for _, company := range companies {
		if normalizeCompanyName(company.GetString("name")) == normalizeCompanyName(name) {
			return nil, fmt.Errorf("an account named %q already exists; select it instead", company.GetString("name"))
		}
	}

	collection, err := app.FindCollectionByNameOrId("companies")
	if err != nil {
		return nil, err
	}
	company := core.NewRecord(collection)
	company.Set("name", name)
	if err := app.Save(company); err != nil {
		return nil, err
	}
	return company, nil
}

func assignCustomerCompany(app core.App, customer *core.Record, companyID string) error {
	wasUnassigned := customer.GetString("company") == ""
	customer.Set("company", companyID)
	if err := app.Save(customer); err != nil {
		return err
	}
	if !wasUnassigned {
		return nil
	}

	// First assignment reconciles only unassigned history. Reassignments never
	// move historical records between accounts.
	for _, collectionName := range []string{"orders", "scheduledOrders"} {
		records, err := app.FindAllRecords(collectionName)
		if err != nil {
			return err
		}
		for _, record := range records {
			if record.GetString("customer") == customer.Id && record.GetString("company") == "" {
				record.Set("company", companyID)
				if err := app.Save(record); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func handleAssignCustomerAccount() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || e.Auth.Collection().Name != "users" {
			return e.ForbiddenError("Only staff can assign wholesale accounts", nil)
		}
		var body accountSelectionBody
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Invalid request body", err)
		}
		company, err := resolveCompany(e.App, body)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}
		customer, err := e.App.FindRecordById("customers", e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("Customer not found", err)
		}
		if err := assignCustomerCompany(e.App, customer, company.Id); err != nil {
			return e.InternalServerError("Could not assign wholesale account", err)
		}
		_ = e.App.ExpandRecord(customer, []string{"company"}, nil)
		customer.IgnoreEmailVisibility(true)
		return e.JSON(http.StatusOK, map[string]any{"customer": customer})
	}
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
		body.Email = strings.TrimSpace(body.Email)
		if body.Email == "" {
			return e.BadRequestError("email is required", nil)
		}
		if err := validateAccountSelection(body.CompanyID, body.NewCompanyName); err != nil {
			return e.BadRequestError(err.Error(), err)
		}

		squareCustomer, err := findSquareCustomer(sq, e, body.Email)
		if err != nil {
			return err
		}
		if existing, _ := e.App.FindAuthRecordByEmail("customers", body.Email); existing != nil {
			return e.BadRequestError("A customer account already exists for this email", nil)
		}
		company, err := resolveCompany(e.App, body.accountSelectionBody)
		if err != nil {
			return e.BadRequestError(err.Error(), err)
		}

		col, err := e.App.FindCollectionByNameOrId("customers")
		if err != nil {
			return e.InternalServerError("customers collection not found", err)
		}
		details := customerDetails(squareCustomer, body.Email)
		record := core.NewRecord(col)
		record.SetEmail(body.Email)
		record.Set("name", details.Name)
		record.Set("phone", details.Phone)
		record.Set("squareCustomerId", details.ID)
		record.Set("company", company.Id)
		record.SetPassword(core.GenerateDefaultRandomId() + core.GenerateDefaultRandomId())

		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("Could not create customer account", err)
		}
		if err := sendCustomerWelcomeEmail(e.App, record); err != nil {
			e.App.Logger().Error("invite: failed to send welcome email",
				"customer_id", record.Id, "email", body.Email, "error", err)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"id": record.Id, "email": body.Email, "name": details.Name,
			"phone": details.Phone, "squareCustomerId": details.ID,
			"company": company.Id,
			"expand":  map[string]any{"company": company},
		})
	}
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
