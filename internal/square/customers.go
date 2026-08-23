package square

import (
	"context"
	"fmt"

	squaresdk "github.com/square/square-go-sdk/v3"
	squarecustomers "github.com/square/square-go-sdk/v3/customers"
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

func (c *Client) validateWholesaleGroupConfig() error {
	if c.WholesaleGroceryGroupID == "" {
		return fmt.Errorf("square: SQUARE_WHOLESALE_GROCERY_GROUP_ID is required")
	}
	if c.WholesaleCafeRestaurantGroupID == "" {
		return fmt.Errorf("square: SQUARE_WHOLESALE_CAFE_RESTAURANT_GROUP_ID is required")
	}
	return nil
}

func (c *Client) wholesaleAudiencesForGroups(groupIDs []string) []WholesaleAudience {
	memberships := make(map[string]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		memberships[groupID] = struct{}{}
	}

	audiences := make([]WholesaleAudience, 0, 2)
	if _, ok := memberships[c.WholesaleGroceryGroupID]; ok {
		audiences = append(audiences, WholesaleAudienceGrocery)
	}
	if _, ok := memberships[c.WholesaleCafeRestaurantGroupID]; ok {
		audiences = append(audiences, WholesaleAudienceCafeRestaurant)
	}
	return audiences
}

// GetCustomerWholesaleAudiences returns the configured wholesale groups for a Square customer.
func (c *Client) GetCustomerWholesaleAudiences(ctx context.Context, customerID string) ([]WholesaleAudience, error) {
	if err := c.validateWholesaleGroupConfig(); err != nil {
		return nil, err
	}
	customer, err := c.GetCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, fmt.Errorf("square: customer %q was not returned", customerID)
	}
	return c.wholesaleAudiencesForGroups(customer.GroupIDs), nil
}

// SetCustomerWholesaleAudiences updates only the configured wholesale group memberships.
func (c *Client) SetCustomerWholesaleAudiences(
	ctx context.Context,
	customerID string,
	grocery bool,
	cafeRestaurant bool,
) ([]WholesaleAudience, error) {
	if err := c.validateWholesaleGroupConfig(); err != nil {
		return nil, err
	}
	customer, err := c.GetCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, fmt.Errorf("square: customer %q was not returned", customerID)
	}

	current := make(map[string]struct{}, len(customer.GroupIDs))
	for _, groupID := range customer.GroupIDs {
		current[groupID] = struct{}{}
	}
	updates := []struct {
		groupID string
		enabled bool
	}{
		{groupID: c.WholesaleGroceryGroupID, enabled: grocery},
		{groupID: c.WholesaleCafeRestaurantGroupID, enabled: cafeRestaurant},
	}
	for _, update := range updates {
		_, enabled := current[update.groupID]
		if enabled == update.enabled {
			continue
		}
		if update.enabled {
			_, err = c.SDK.Customers.Groups.Add(ctx, &squarecustomers.AddGroupsRequest{
				CustomerID: customerID,
				GroupID:    update.groupID,
			})
		} else {
			_, err = c.SDK.Customers.Groups.Remove(ctx, &squarecustomers.RemoveGroupsRequest{
				CustomerID: customerID,
				GroupID:    update.groupID,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("square: update customer %q group %q: %w", customerID, update.groupID, err)
		}
	}

	audiences := make([]WholesaleAudience, 0, 2)
	if grocery {
		audiences = append(audiences, WholesaleAudienceGrocery)
	}
	if cafeRestaurant {
		audiences = append(audiences, WholesaleAudienceCafeRestaurant)
	}
	return audiences, nil
}
