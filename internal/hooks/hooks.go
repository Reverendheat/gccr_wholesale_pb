package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/reverendheat/gccr_invoice/internal/billing"
	squaresdk "github.com/square/square-go-sdk/v3"
)

// squareCustomerGetter is satisfied by *square.Client and allows test mocking.
type squareCustomerGetter interface {
	GetCustomer(ctx context.Context, id string) (*squaresdk.Customer, error)
}

// Register wires all application hooks onto app.
func Register(app core.App, sq squareCustomerGetter) {
	// RequestInfo.Body already contains PocketBase's coerced field values here.
	// Rebind the original body so JSON strings cannot masquerade as email lists.
	validateBillingRequest := func(e *core.RecordRequestEvent) error {
		var body map[string]any
		if err := e.BindBody(&body); err != nil {
			return apis.NewBadRequestError("Invalid billing request", err)
		}
		if value, exists := body["billingEmails"]; exists {
			raw, err := json.Marshal(value)
			if err != nil {
				return apis.NewBadRequestError("Invalid billing emails", err)
			}
			if _, err := billing.NormalizeEmails(raw); err != nil {
				return apis.NewBadRequestError("Invalid billing emails", err)
			}
		}
		return e.Next()
	}
	app.OnRecordCreateRequest("companies").BindFunc(validateBillingRequest)
	app.OnRecordUpdateRequest("companies").BindFunc(validateBillingRequest)

	// Model validation also covers direct collection and superuser writes.
	app.OnRecordValidate("companies").BindFunc(func(e *core.RecordEvent) error {
		emails, err := billing.NormalizeEmails([]byte(e.Record.GetString("billingEmails")))
		if err != nil {
			return apis.NewBadRequestError("Invalid billing emails", err)
		}
		e.Record.Set("billingEmails", emails)
		return e.Next()
	})

	// Gate customer record creation: the squareCustomerId must exist in Square.
	app.OnRecordCreateRequest("customers").BindFunc(func(e *core.RecordRequestEvent) error {
		squareID := e.Record.GetString("squareCustomerId")
		if err := validateSquareCustomer(e.Request.Context(), sq, squareID); err != nil {
			return apis.NewBadRequestError(err.Error(), nil)
		}
		return e.Next()
	})

	// Belt-and-suspenders: block login if squareCustomerId is somehow missing.
	app.OnRecordAuthRequest("customers").BindFunc(func(e *core.RecordAuthRequestEvent) error {
		if e.Record.GetString("squareCustomerId") == "" {
			return apis.NewForbiddenError("account is not linked to a Square customer", nil)
		}
		return e.Next()
	})
}

// validateSquareCustomer confirms the given Square customer ID resolves to a real customer.
// Extracted for unit testability.
func validateSquareCustomer(ctx context.Context, sq squareCustomerGetter, squareID string) error {
	if squareID == "" {
		return fmt.Errorf("squareCustomerId is required")
	}
	if _, err := sq.GetCustomer(ctx, squareID); err != nil {
		return fmt.Errorf("could not verify Square customer: %w", err)
	}
	return nil
}
