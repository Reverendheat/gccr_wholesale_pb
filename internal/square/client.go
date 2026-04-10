// Package square wraps the Square API for invoice operations.
package square

import (
	squaresdk "github.com/square/square-go-sdk/v3"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

// Client wraps the official Square Go SDK client.
type Client struct {
	SDK *squareclient.Client
}

// Config holds Square API configuration loaded from environment variables.
type Config struct {
	AccessToken string
	Sandbox     bool
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
		SDK: squareclient.NewClient(opts...),
	}
}
