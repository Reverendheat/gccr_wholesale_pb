package hooks

import (
	"context"
	"fmt"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	squaresdk "github.com/square/square-go-sdk/v3"
)

// squareCustomerGetter is satisfied by *square.Client and allows test mocking.
type squareCustomerGetter interface {
	GetCustomer(ctx context.Context, id string) (*squaresdk.Customer, error)
}

// Register wires all application hooks onto app.
func Register(app core.App, sq squareCustomerGetter) {
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
