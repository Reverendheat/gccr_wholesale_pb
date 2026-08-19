package delivery

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func PolicyFromEnv() (Policy, error) {
	maxMiles, err := envFloat("DELIVERY_MAX_MILES", 30)
	if err != nil {
		return Policy{}, err
	}
	freeMinimum, err := envInt64("DELIVERY_FREE_MINIMUM_CENTS", 10000)
	if err != nil {
		return Policy{}, err
	}
	rate, err := envInt64("DELIVERY_RATE_CENTS_PER_MILE", 50)
	if err != nil {
		return Policy{}, err
	}
	if maxMiles <= 0 || freeMinimum < 0 || rate < 0 {
		return Policy{}, fmt.Errorf("delivery policy values must be non-negative and max miles must be positive")
	}
	return Policy{MaxMiles: maxMiles, FreeMinimumCents: freeMinimum, RateCentsPerMile: rate}, nil
}

func envFloat(key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", key, err)
	}
	return value, nil
}

func envInt64(key string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}
