// Package billing validates invoice recipient configuration.
package billing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
)

const MaxEmailListBytes = 64 * 1024

// NormalizeEmails accepts a JSON string array or null (no override). It never
// coerces scalars or array entries into addresses.
func NormalizeEmails(raw []byte) ([]string, error) {
	result := []string{}
	if len(raw) > MaxEmailListBytes {
		return nil, fmt.Errorf("email list exceeds %d bytes", MaxEmailListBytes)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return result, nil
	}
	if raw[0] != '[' {
		return nil, fmt.Errorf("email list must be an array of email addresses")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("invalid email list: %w", err)
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		var value string
		if len(entry) == 0 || entry[0] != '"' || json.Unmarshal(entry, &value) != nil {
			return nil, fmt.Errorf("email list entries must be strings")
		}
		value = strings.TrimSpace(value)
		address, err := mail.ParseAddress(value)
		if err != nil || value == "" || len(value) > 254 || address.Address != value || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("invalid email address %q", value)
		}
		value = strings.ToLower(value)
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, nil
}
