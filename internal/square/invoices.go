package square

import (
	"context"
	"fmt"

	squaresdk "github.com/square/square-go-sdk/v3"
)

// CreateAndPublishInvoice creates a Square invoice for the given order and
// immediately publishes it so Square emails the customer with a payment link.
// dueDate must be in YYYY-MM-DD format.
func (c *Client) CreateAndPublishInvoice(
	ctx context.Context,
	squareOrderID, locationID, squareCustomerID string,
	dueDate string,
	idempotencyKeySuffix string,
) (*squaresdk.Invoice, error) {
	if squareOrderID == "" {
		return nil, fmt.Errorf("square: squareOrderID is required")
	}
	if squareCustomerID == "" {
		return nil, fmt.Errorf("square: squareCustomerID is required")
	}

	createResp, err := c.SDK.Invoices.Create(ctx, &squaresdk.CreateInvoiceRequest{
		IdempotencyKey: squaresdk.String("create-inv-" + idempotencyKeySuffix),
		Invoice: &squaresdk.Invoice{
			OrderID:    squaresdk.String(squareOrderID),
			LocationID: squaresdk.String(locationID),
			PrimaryRecipient: &squaresdk.InvoiceRecipient{
				CustomerID: squaresdk.String(squareCustomerID),
			},
			PaymentRequests: []*squaresdk.InvoicePaymentRequest{
				{
					RequestType: squaresdk.InvoiceRequestTypeBalance.Ptr(),
					DueDate:     squaresdk.String(dueDate),
				},
			},
			DeliveryMethod: squaresdk.InvoiceDeliveryMethodEmail.Ptr(),
			AcceptedPaymentMethods: &squaresdk.InvoiceAcceptedPaymentMethods{
				Card:        squaresdk.Bool(true),
				BankAccount: squaresdk.Bool(true),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("square: CreateInvoice: %w", err)
	}

	inv := createResp.Invoice

	publishResp, err := c.SDK.Invoices.Publish(ctx, &squaresdk.PublishInvoiceRequest{
		InvoiceID:      *inv.ID,
		Version:        *inv.Version,
		IdempotencyKey: squaresdk.String("pub-inv-" + idempotencyKeySuffix),
	})
	if err != nil {
		return nil, fmt.Errorf("square: PublishInvoice: %w", err)
	}

	return publishResp.Invoice, nil
}

// GetInvoice fetches a single invoice by its Square invoice ID.
func (c *Client) GetInvoice(ctx context.Context, invoiceID string) (*squaresdk.Invoice, error) {
	if invoiceID == "" {
		return nil, fmt.Errorf("square: invoiceID is required")
	}

	resp, err := c.SDK.Invoices.Get(ctx, &squaresdk.GetInvoicesRequest{
		InvoiceID: invoiceID,
	})
	if err != nil {
		return nil, fmt.Errorf("square: GetInvoice %q: %w", invoiceID, err)
	}

	return resp.Invoice, nil
}
