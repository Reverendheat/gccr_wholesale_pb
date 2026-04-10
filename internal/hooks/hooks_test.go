package hooks

import (
	"context"
	"errors"
	"testing"

	squaresdk "github.com/square/square-go-sdk/v3"
)

// mockSquare implements squareCustomerGetter for tests.
type mockSquare struct {
	customer *squaresdk.Customer
	err      error
}

func (m *mockSquare) GetCustomer(_ context.Context, _ string) (*squaresdk.Customer, error) {
	return m.customer, m.err
}

func TestValidateSquareCustomer_EmptyID(t *testing.T) {
	sq := &mockSquare{customer: &squaresdk.Customer{ID: squaresdk.String("X")}}
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
	sq := &mockSquare{customer: &squaresdk.Customer{ID: squaresdk.String("VALID123")}}
	err := validateSquareCustomer(context.Background(), sq, "VALID123")
	if err != nil {
		t.Fatalf("expected no error for valid customer, got: %v", err)
	}
}
