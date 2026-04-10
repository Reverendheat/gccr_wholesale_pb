package square

import (
	"context"
	"fmt"

	squaresdk "github.com/square/square-go-sdk/v3"
)

// SearchCustomerByEmail looks up a Square customer by exact email address.
// Returns nil, nil if no customer is found.
func (c *Client) SearchCustomerByEmail(ctx context.Context, email string) (*squaresdk.Customer, error) {
	if email == "" {
		return nil, fmt.Errorf("square: email must not be empty")
	}

	resp, err := c.SDK.Customers.Search(ctx, &squaresdk.SearchCustomersRequest{
		Query: &squaresdk.CustomerQuery{
			Filter: &squaresdk.CustomerFilter{
				EmailAddress: &squaresdk.CustomerTextFilter{
					Exact: squaresdk.String(email),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("square: SearchCustomerByEmail: %w", err)
	}

	if len(resp.Customers) == 0 {
		return nil, nil
	}
	return resp.Customers[0], nil
}

// GetCustomer fetches a single customer by their Square customer ID.
func (c *Client) GetCustomer(ctx context.Context, customerID string) (*squaresdk.Customer, error) {
	if customerID == "" {
		return nil, fmt.Errorf("square: customer ID must not be empty")
	}

	resp, err := c.SDK.Customers.Get(ctx, &squaresdk.GetCustomersRequest{
		CustomerID: customerID,
	})
	if err != nil {
		return nil, fmt.Errorf("square: GetCustomer %q: %w", customerID, err)
	}

	return resp.Customer, nil
}
