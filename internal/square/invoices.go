package square

import (
	"context"
	"fmt"

	squaresdk "github.com/square/square-go-sdk/v3"
)

// CreateInvoice creates a manually shared draft. The portal, not Square, emails
// its payment link. Callers must persist the draft ID before publishing it.
func (c *Client) CreateInvoice(ctx context.Context, squareOrderID, locationID, squareCustomerID, dueDate, idempotencyKeySuffix string) (*squaresdk.Invoice, error) {
	if squareOrderID == "" || squareCustomerID == "" || locationID == "" || dueDate == "" {
		return nil, fmt.Errorf("square: order, location, customer and due date are required")
	}
	resp, err := c.SDK.Invoices.Create(ctx, &squaresdk.CreateInvoiceRequest{
		IdempotencyKey: squaresdk.String("create-inv-" + idempotencyKeySuffix),
		Invoice: &squaresdk.Invoice{
			OrderID:          squaresdk.String(squareOrderID),
			LocationID:       squaresdk.String(locationID),
			PrimaryRecipient: &squaresdk.InvoiceRecipient{CustomerID: squaresdk.String(squareCustomerID)},
			PaymentRequests: []*squaresdk.InvoicePaymentRequest{{
				RequestType: squaresdk.InvoiceRequestTypeBalance.Ptr(),
				DueDate:     squaresdk.String(dueDate),
			}},
			DeliveryMethod: squaresdk.InvoiceDeliveryMethodShareManually.Ptr(),
			AcceptedPaymentMethods: &squaresdk.InvoiceAcceptedPaymentMethods{
				Card: squaresdk.Bool(true), BankAccount: squaresdk.Bool(true),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("square: CreateInvoice: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("square: CreateInvoice returned no response")
	}
	return validInvoice(resp.Invoice)
}

func (c *Client) PublishInvoice(ctx context.Context, invoice *squaresdk.Invoice, idempotencyKeySuffix string) (*squaresdk.Invoice, error) {
	if _, err := validInvoice(invoice); err != nil {
		return nil, err
	}
	resp, err := c.SDK.Invoices.Publish(ctx, &squaresdk.PublishInvoiceRequest{
		InvoiceID: *invoice.ID,
		Version:   *invoice.Version,
		// Version-specific keys allow a deliberate external draft edit without
		// reusing an idempotency key with a different publication payload.
		IdempotencyKey: squaresdk.String(fmt.Sprintf("pub-inv-%s-%d", idempotencyKeySuffix, *invoice.Version)),
	})
	if err != nil {
		return nil, fmt.Errorf("square: PublishInvoice: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("square: PublishInvoice returned no response")
	}
	return validInvoice(resp.Invoice)
}

// GetInvoice fetches current status and a fresh hosted payment URL on retries.
func (c *Client) GetInvoice(ctx context.Context, invoiceID string) (*squaresdk.Invoice, error) {
	if invoiceID == "" {
		return nil, fmt.Errorf("square: invoiceID is required")
	}
	resp, err := c.SDK.Invoices.Get(ctx, &squaresdk.GetInvoicesRequest{InvoiceID: invoiceID})
	if err != nil {
		return nil, fmt.Errorf("square: GetInvoice %q: %w", invoiceID, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("square: GetInvoice returned no response")
	}
	invoice, err := validInvoice(resp.Invoice)
	if err == nil && *invoice.ID != invoiceID {
		return nil, fmt.Errorf("square: GetInvoice returned a different invoice")
	}
	return invoice, err
}

func validInvoice(invoice *squaresdk.Invoice) (*squaresdk.Invoice, error) {
	if invoice == nil || invoice.ID == nil || *invoice.ID == "" || invoice.Version == nil || invoice.Status == nil {
		return nil, fmt.Errorf("square: invoice response is missing ID, version or status")
	}
	return invoice, nil
}
