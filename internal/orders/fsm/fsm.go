// Package fsm defines the local order workflow rules.
package fsm

import "fmt"

type Status string

const (
	StatusPending     Status = "pending"
	StatusConfirmed   Status = "confirmed"
	StatusDelivered   Status = "delivered"
	StatusInvoiced    Status = "invoiced"
	StatusPaid        Status = "paid"
	StatusCancelled   Status = "cancelled"
	StatusNeedsReview Status = "needs_review"
)

type Event string

const (
	EventStaffConfirm       Event = "staff_confirm"
	EventStaffMarkDelivered Event = "staff_mark_delivered"
	EventStaffSendInvoice   Event = "staff_send_invoice"
	EventStaffCancel        Event = "staff_cancel"
	EventCustomerCancel     Event = "customer_cancel"
	EventCustomerChangeReq  Event = "customer_change_request"
	EventSquareInvoicePaid  Event = "square_invoice_paid"
	EventSquareNeedsReview  Event = "square_needs_review"
)

var transitions = map[Status]map[Event]Status{
	StatusPending: {
		EventStaffConfirm:      StatusConfirmed,
		EventStaffSendInvoice:  StatusInvoiced,
		EventStaffCancel:       StatusCancelled,
		EventCustomerCancel:    StatusCancelled,
		EventCustomerChangeReq: StatusPending,
		EventSquareInvoicePaid: StatusPaid,
		EventSquareNeedsReview: StatusNeedsReview,
	},
	StatusConfirmed: {
		EventStaffMarkDelivered: StatusDelivered,
		EventStaffSendInvoice:   StatusInvoiced,
		EventStaffCancel:        StatusCancelled,
		EventSquareInvoicePaid:  StatusPaid,
		EventSquareNeedsReview:  StatusNeedsReview,
	},
	StatusDelivered: {
		EventStaffSendInvoice:  StatusInvoiced,
		EventStaffCancel:       StatusCancelled,
		EventSquareInvoicePaid: StatusPaid,
		EventSquareNeedsReview: StatusNeedsReview,
	},
	StatusInvoiced: {
		EventSquareInvoicePaid: StatusPaid,
		EventSquareNeedsReview: StatusNeedsReview,
	},
	StatusPaid: {
		EventSquareInvoicePaid: StatusPaid,
	},
	StatusCancelled: {
		EventSquareInvoicePaid: StatusNeedsReview,
		EventSquareNeedsReview: StatusNeedsReview,
	},
	StatusNeedsReview: {
		EventStaffCancel: StatusCancelled,
	},
}

// Apply returns the next status for an allowed transition.
func Apply(status Status, event Event) (Status, error) {
	next, ok := transitions[status][event]
	if !ok {
		return "", fmt.Errorf("cannot apply %q to %q", event, status)
	}
	return next, nil
}

func IsTerminal(status Status) bool {
	return status == StatusPaid || status == StatusCancelled
}
