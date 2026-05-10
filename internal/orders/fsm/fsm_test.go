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
		{"customer change request keeps pending", StatusPending, EventCustomerChangeReq, StatusPending},
		{"square payment moves invoiced to paid", StatusInvoiced, EventSquareInvoicePaid, StatusPaid},
		{"square payment can pull pending to paid", StatusPending, EventSquareInvoicePaid, StatusPaid},
		{"duplicate square payment is idempotent", StatusPaid, EventSquareInvoicePaid, StatusPaid},
		{"square payment on cancelled needs review", StatusCancelled, EventSquareInvoicePaid, StatusNeedsReview},
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
