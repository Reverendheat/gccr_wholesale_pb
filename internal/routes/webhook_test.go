package routes

import (
	"testing"

	"github.com/reverendheat/gccr_invoice/internal/orders/fsm"
)

func TestSquareInvoiceEvent(t *testing.T) {
	tests := []struct {
		eventType string
		want      fsm.Event
		ok        bool
	}{
		{"invoice.payment_made", fsm.EventSquareInvoicePaid, true},
		{"invoice.canceled", fsm.EventSquareInvoiceCancelled, true},
		{"invoice.refunded", fsm.EventSquareNeedsReview, true},
		{"invoice.updated", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			got, ok := squareInvoiceEvent(tt.eventType)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}
