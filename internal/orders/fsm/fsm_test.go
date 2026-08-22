package fsm

import "testing"

func TestApplyAllowedTransitions(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		event  Event
		want   Status
	}{
		{"staff confirms pending", StatusPending, EventStaffConfirm, StatusConfirmed},
		{"staff delivers confirmed", StatusConfirmed, EventStaffMarkDelivered, StatusDelivered},
		{"staff invoices pending", StatusPending, EventStaffSendInvoice, StatusInvoiced},
		{"staff invoices confirmed", StatusConfirmed, EventStaffSendInvoice, StatusInvoiced},
		{"staff invoices delivered", StatusDelivered, EventStaffSendInvoice, StatusInvoiced},
		{"customer cancels pending", StatusPending, EventCustomerCancel, StatusCancelled},
		{"customer change request keeps pending", StatusPending, EventCustomerChangeReq, StatusPending},
		{"square payment moves invoiced to paid", StatusInvoiced, EventSquareInvoicePaid, StatusPaid},
		{"square payment can pull pending to paid", StatusPending, EventSquareInvoicePaid, StatusPaid},
		{"duplicate square payment is idempotent", StatusPaid, EventSquareInvoicePaid, StatusPaid},
		{"square payment on cancelled needs review", StatusCancelled, EventSquareInvoicePaid, StatusNeedsReview},
		{"square cancellation cancels invoiced order", StatusInvoiced, EventSquareInvoiceCancelled, StatusCancelled},
		{"duplicate square cancellation is idempotent", StatusCancelled, EventSquareInvoiceCancelled, StatusCancelled},
		{"square refund after payment needs review", StatusPaid, EventSquareNeedsReview, StatusNeedsReview},
		{"duplicate review event is idempotent", StatusNeedsReview, EventSquareNeedsReview, StatusNeedsReview},
		{"needs review can be cancelled by staff", StatusNeedsReview, EventStaffCancel, StatusCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(tt.status, tt.event)
			if err != nil {
				t.Fatalf("Apply returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Apply() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEventForSquareInvoiceStatus(t *testing.T) {
	tests := []struct {
		status string
		want   Event
		ok     bool
	}{
		{"PAID", EventSquareInvoicePaid, true},
		{"CANCELED", EventSquareInvoiceCancelled, true},
		{"PARTIALLY_PAID", EventSquareNeedsReview, true},
		{"REFUNDED", EventSquareNeedsReview, true},
		{"FAILED", EventSquareNeedsReview, true},
		{"UNPAID", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got, ok := EventForSquareInvoiceStatus(tt.status)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestApplyDeniedTransitions(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		event  Event
	}{
		{"customer cannot change confirmed", StatusConfirmed, EventCustomerChangeReq},
		{"paid is terminal", StatusPaid, EventStaffCancel},
		{"cancelled is terminal", StatusCancelled, EventStaffConfirm},
		{"cannot deliver pending", StatusPending, EventStaffMarkDelivered},
		{"cannot confirm invoiced", StatusInvoiced, EventStaffConfirm},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := Apply(tt.status, tt.event); err == nil {
				t.Fatalf("Apply() = %q, want error", got)
			}
		})
	}
}
