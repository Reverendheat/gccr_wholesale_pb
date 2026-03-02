package square

import (
	"context"
	"fmt"
	"net/url"
)

type searchCustomersRequest struct {
	Query struct {
		Filter struct {
			EmailAddress struct {
				Exact string `json:"exact"`
			} `json:"email_address"`
		} `json:"filter"`
	} `json:"query"`
}

type searchCustomersResponse struct {
	Customers []Customer `json:"customers"`
}

// Customer holds the Square fields relevant to this app.
type Customer struct {
	ID           string `json:"id"`
	GivenName    string `json:"given_name"`
	FamilyName   string `json:"family_name"`
	EmailAddress string `json:"email_address"`
	PhoneNumber  string `json:"phone_number"`
}

type getCustomerResponse struct {
	Customer Customer `json:"customer"`
}

// SearchCustomerByEmail looks up a Square customer by exact email address.
// Returns nil, nil if no customer is found.
func (c *Client) SearchCustomerByEmail(ctx context.Context, email string) (*Customer, error) {
	if email == "" {
		return nil, fmt.Errorf("square: email must not be empty")
	}

	var req searchCustomersRequest
	req.Query.Filter.EmailAddress.Exact = email

	var resp searchCustomersResponse
	if err := c.doPost(ctx, "/v2/customers/search", req, &resp); err != nil {
		return nil, fmt.Errorf("square: SearchCustomerByEmail: %w", err)
	}

	if len(resp.Customers) == 0 {
		return nil, nil
	}
	return &resp.Customers[0], nil
}

// GetCustomer fetches a single customer by their Square customer ID.
// Returns an error if the customer does not exist or the ID is invalid.
func (c *Client) GetCustomer(ctx context.Context, customerID string) (*Customer, error) {
	if customerID == "" {
		return nil, fmt.Errorf("square: customer ID must not be empty")
	}

	// Escape the ID so it is safe to use as a URL path segment.
	safePath := "/v2/customers/" + url.PathEscape(customerID)

	var resp getCustomerResponse
	if err := c.doGet(ctx, safePath, &resp); err != nil {
		return nil, fmt.Errorf("square: GetCustomer %q: %w", customerID, err)
	}

	return &resp.Customer, nil
}
