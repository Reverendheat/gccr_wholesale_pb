package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// ApplySettingsFromEnv persists PocketBase runtime settings from environment
// variables so deployments don't depend on hand-edited admin panel values.
func ApplySettingsFromEnv(app core.App) error {
	settings := app.Settings()

	setString := func(target *string, key string) {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			*target = value
		}
	}

	setString(&settings.Meta.AppName, "PB_APP_NAME")
	setString(&settings.Meta.AppURL, "PB_APP_URL")
	settings.Meta.AppURL = strings.TrimRight(settings.Meta.AppURL, "/")
	setString(&settings.Meta.SenderName, "PB_SENDER_NAME")
	setString(&settings.Meta.SenderAddress, "PB_SENDER_ADDRESS")

	if value, ok, err := envBool("PB_SMTP_ENABLED"); err != nil {
		return err
	} else if ok {
		settings.SMTP.Enabled = value
	}

	setString(&settings.SMTP.Host, "PB_SMTP_HOST")
	setString(&settings.SMTP.Username, "PB_SMTP_USERNAME")
	setString(&settings.SMTP.Password, "PB_SMTP_PASSWORD")
	setString(&settings.SMTP.AuthMethod, "PB_SMTP_AUTH_METHOD")
	setString(&settings.SMTP.LocalName, "PB_SMTP_LOCAL_NAME")

	if value, ok, err := envInt("PB_SMTP_PORT"); err != nil {
		return err
	} else if ok {
		settings.SMTP.Port = value
	}

	if value, ok, err := envBool("PB_SMTP_TLS"); err != nil {
		return err
	} else if ok {
		settings.SMTP.TLS = value
	}

	if os.Getenv("PB_SMTP_ENABLED") == "" && settings.SMTP.Host != "" {
		settings.SMTP.Enabled = true
	}

	return app.Save(settings)
}

func envBool(key string) (bool, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false, false, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, true, fmt.Errorf("%s must be a boolean: %w", key, err)
	}

	return value, true, nil
}

func envInt(key string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, true, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	return value, true, nil
}
