package square

import (
	"context"
	"fmt"
)

// Invoice is a minimal representation of a Square Invoice.
type Invoice struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Status    string `json:"status"`
	PublicURL string `json:"public_url"`
}

type createInvoiceRequest struct {
	Invoice        invoiceBody `json:"invoice"`
	IdempotencyKey string      `json:"idempotency_key"`
}

type invoiceBody struct {
	OrderID                string                  `json:"order_id"`
	LocationID             string                  `json:"location_id"`
	PrimaryRecipient       invoiceRecipient        `json:"primary_recipient"`
	PaymentRequests        []invoicePaymentRequest `json:"payment_requests"`
	DeliveryMethod         string                  `json:"delivery_method"`
	AcceptedPaymentMethods invoicePaymentMethods   `json:"accepted_payment_methods"`
}

type invoiceRecipient struct {
	CustomerID string `json:"customer_id"`
}

type invoicePaymentRequest struct {
	RequestType string `json:"request_type"`
	DueDate     string `json:"due_date"`
}

type invoicePaymentMethods struct {
	Card        bool `json:"card"`
	BankAccount bool `json:"bank_account"`
}

type createInvoiceResponse struct {
	Invoice Invoice `json:"invoice"`
}

type publishInvoiceRequest struct {
	Version        int    `json:"version"`
	IdempotencyKey string `json:"idempotency_key"`
}

type publishInvoiceResponse struct {
	Invoice Invoice `json:"invoice"`
}

// CreateAndPublishInvoice creates a Square invoice for the given order and
// immediately publishes it so Square emails the customer with a payment link.
// dueDate must be in YYYY-MM-DD format.
// idempotencyKeySuffix should be a unique value per attempt (e.g. PocketBase order ID).
func (c *Client) CreateAndPublishInvoice(
	ctx context.Context,
	squareOrderID, locationID, squareCustomerID string,
	dueDate string,
	idempotencyKeySuffix string,
) (*Invoice, error) {
	if squareOrderID == "" {
		return nil, fmt.Errorf("square: squareOrderID is required")
	}
	if squareCustomerID == "" {
		return nil, fmt.Errorf("square: squareCustomerID is required")
	}

	createReq := createInvoiceRequest{
		IdempotencyKey: "create-inv-" + idempotencyKeySuffix,
		Invoice: invoiceBody{
			OrderID:    squareOrderID,
			LocationID: locationID,
			PrimaryRecipient: invoiceRecipient{
				CustomerID: squareCustomerID,
			},
			PaymentRequests: []invoicePaymentRequest{
				{
					RequestType: "BALANCE",
					DueDate:     dueDate,
				},
			},
			DeliveryMethod: "EMAIL",
			AcceptedPaymentMethods: invoicePaymentMethods{
				Card:        true,
				BankAccount: true,
			},
		},
	}

	var createResp createInvoiceResponse
	if err := c.doPost(ctx, "/v2/invoices", createReq, &createResp); err != nil {
		return nil, fmt.Errorf("square: CreateInvoice: %w", err)
	}
	inv := &createResp.Invoice

	// Publish immediately so Square delivers the invoice email to the customer.
	publishReq := publishInvoiceRequest{
		Version:        inv.Version,
		IdempotencyKey: "pub-inv-" + idempotencyKeySuffix,
	}
	var publishResp publishInvoiceResponse
	if err := c.doPost(ctx, fmt.Sprintf("/v2/invoices/%s/publish", inv.ID), publishReq, &publishResp); err != nil {
		return nil, fmt.Errorf("square: PublishInvoice: %w", err)
	}

	return &publishResp.Invoice, nil
}
