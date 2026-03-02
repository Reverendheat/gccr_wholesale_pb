package hooks

import (
	"context"
	"errors"
	"testing"

	"github.com/reverendheat/gccr_invoice/internal/square"
)

// mockSquare implements squareCustomerGetter for tests.
type mockSquare struct {
	customer *square.Customer
	err      error
}

func (m *mockSquare) GetCustomer(_ context.Context, _ string) (*square.Customer, error) {
	return m.customer, m.err
}

func TestValidateSquareCustomer_EmptyID(t *testing.T) {
	sq := &mockSquare{customer: &square.Customer{ID: "X"}}
	err := validateSquareCustomer(context.Background(), sq, "")
	if err == nil {
		t.Fatal("expected error for empty square_customer_id")
	}
}

func TestValidateSquareCustomer_SquareNotFound(t *testing.T) {
	sq := &mockSquare{err: errors.New("square: unexpected status 404")}
	err := validateSquareCustomer(context.Background(), sq, "DOESNOTEXIST")
	if err == nil {
		t.Fatal("expected error when Square returns not found")
	}
}

func TestValidateSquareCustomer_Valid(t *testing.T) {
	sq := &mockSquare{customer: &square.Customer{ID: "VALID123"}}
	err := validateSquareCustomer(context.Background(), sq, "VALID123")
	if err != nil {
		t.Fatalf("expected no error for valid customer, got: %v", err)
	}
}
