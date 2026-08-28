// Package square wraps the Square API for invoice operations.
package square

import (
	squaresdk "github.com/square/square-go-sdk/v3"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

// Client wraps the official Square Go SDK client.
type Client struct {
	SDK                                   *squareclient.Client
	WholesaleCategoryID                   string
	WholesaleGroceryGroupID               string
	WholesaleCafeRestaurantGroupID        string
	WholesaleGroceryAttributeID           string
	WholesaleCafeRestaurantAttributeID    string
	WholesaleCustomerAllowlistAttributeID string
	WholesaleGrindModifierListID          string
	WholesaleDripModifierID               string
}

// Config holds Square API configuration loaded from environment variables.
type Config struct {
	AccessToken                           string
	Sandbox                               bool
	WholesaleCategoryID                   string
	WholesaleGroceryGroupID               string
	WholesaleCafeRestaurantGroupID        string
	WholesaleGroceryAttributeID           string
	WholesaleCafeRestaurantAttributeID    string
	WholesaleCustomerAllowlistAttributeID string
	WholesaleGrindModifierListID          string
	WholesaleDripModifierID               string
}

// New creates a new Square Client from the provided config.
func New(cfg Config) *Client {
	opts := []option.RequestOption{
		option.WithToken(cfg.AccessToken),
	}
	if cfg.Sandbox {
		opts = append(opts, option.WithBaseURL(squaresdk.Environments.Sandbox))
	}
	return &Client{
		SDK:                                   squareclient.NewClient(opts...),
		WholesaleCategoryID:                   cfg.WholesaleCategoryID,
		WholesaleGroceryGroupID:               cfg.WholesaleGroceryGroupID,
		WholesaleCafeRestaurantGroupID:        cfg.WholesaleCafeRestaurantGroupID,
		WholesaleGroceryAttributeID:           cfg.WholesaleGroceryAttributeID,
		WholesaleCafeRestaurantAttributeID:    cfg.WholesaleCafeRestaurantAttributeID,
		WholesaleCustomerAllowlistAttributeID: cfg.WholesaleCustomerAllowlistAttributeID,
		WholesaleGrindModifierListID:          cfg.WholesaleGrindModifierListID,
		WholesaleDripModifierID:               cfg.WholesaleDripModifierID,
	}
}
