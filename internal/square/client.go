// Package square wraps the Square API for invoice operations.
package square

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

const (
	productionBaseURL = "https://connect.squareup.com"
	sandboxBaseURL    = "https://connect.squareupsandbox.com"
)

// Client is an HTTP client for the Square API.
type Client struct {
	baseURL    string
	accessToken string
	http       *http.Client
}

// Config holds Square API configuration loaded from environment variables.
type Config struct {
	AccessToken string
	Sandbox     bool
}

// New creates a new Square Client from the provided config.
func New(cfg Config) *Client {
	base := productionBaseURL
	if cfg.Sandbox {
		base = sandboxBaseURL
	}
	return &Client{
		baseURL:     base,
		accessToken: cfg.AccessToken,
		http:        &http.Client{},
	}
}

// doGet performs an authenticated GET request and decodes the JSON response.
func (c *Client) doGet(ctx context.Context, path string, out any) error {
	slog.DebugContext(ctx, "square GET", "path", path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("square: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Square-Version", "2024-04-17")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("square: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		slog.ErrorContext(ctx, "square GET error", "path", path, "status", resp.StatusCode, "body", string(errBody))
		return fmt.Errorf("square: unexpected status %d: %s", resp.StatusCode, errBody)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// doPost performs an authenticated POST request and decodes the JSON response.
func (c *Client) doPost(ctx context.Context, path string, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("square: marshal request: %w", err)
	}

	slog.DebugContext(ctx, "square POST", "path", path, "body", string(body))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("square: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Square-Version", "2024-04-17")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("square: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		errBody, _ := io.ReadAll(resp.Body)
		slog.ErrorContext(ctx, "square POST error", "path", path, "status", resp.StatusCode, "body", string(errBody), "request_body", string(body))
		return fmt.Errorf("square: unexpected status %d: %s", resp.StatusCode, errBody)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
