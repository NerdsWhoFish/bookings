package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port               string
	DevMode            bool
	ProjectID          string
	FirestoreDatabase  string
	KMSKeyName         string
	PublicURL          string
	Theme              string
	AdminEmails        map[string]bool
	GoogleClientID     string
	GoogleClientSecret string
	SessionKey         string
	TurnstileSiteKey   string
	TurnstileSecret    string
	ExternalBlockToken string
	FaroURL            string
	FaroAppName        string
}

func Load() (Config, error) {
	result := Config{
		Port:               value("PORT", "8080"),
		DevMode:            os.Getenv("BOOKINGS_DEV_MODE") == "true",
		ProjectID:          os.Getenv("BOOKINGS_GCP_PROJECT_ID"),
		FirestoreDatabase:  value("BOOKINGS_FIRESTORE_DATABASE", "(default)"),
		KMSKeyName:         os.Getenv("BOOKINGS_KMS_KEY_NAME"),
		PublicURL:          value("BOOKINGS_PUBLIC_URL", "http://localhost:8080"),
		Theme:              value("BOOKINGS_THEME", "nerdswhofish"),
		AdminEmails:        emailSet(os.Getenv("BOOKINGS_ADMIN_EMAILS")),
		GoogleClientID:     os.Getenv("BOOKINGS_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("BOOKINGS_GOOGLE_CLIENT_SECRET"),
		SessionKey:         os.Getenv("BOOKINGS_SESSION_KEY"),
		TurnstileSiteKey:   os.Getenv("BOOKINGS_TURNSTILE_SITE_KEY"),
		TurnstileSecret:    os.Getenv("BOOKINGS_TURNSTILE_SECRET"),
		ExternalBlockToken: os.Getenv("BOOKINGS_EXTERNAL_BLOCK_TOKEN"),
		FaroURL:            os.Getenv("BOOKINGS_FARO_URL"),
		FaroAppName:        value("BOOKINGS_FARO_APP_NAME", "bookings"),
	}
	if result.DevMode {
		return result, nil
	}
	missing := make([]string, 0, 7)
	for name, configured := range map[string]bool{
		"BOOKINGS_GCP_PROJECT_ID":       result.ProjectID != "",
		"BOOKINGS_KMS_KEY_NAME":         result.KMSKeyName != "",
		"BOOKINGS_GOOGLE_CLIENT_ID":     result.GoogleClientID != "",
		"BOOKINGS_GOOGLE_CLIENT_SECRET": result.GoogleClientSecret != "",
		"BOOKINGS_SESSION_KEY":          len(result.SessionKey) >= 32,
		"BOOKINGS_ADMIN_EMAILS":         len(result.AdminEmails) > 0,
		"BOOKINGS_TURNSTILE_SECRET":     result.TurnstileSecret != "",
	} {
		if !configured {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing production configuration: %s", strings.Join(missing, ", "))
	}
	if result.ExternalBlockToken != "" && len(result.ExternalBlockToken) < 32 {
		return Config{}, fmt.Errorf("BOOKINGS_EXTERNAL_BLOCK_TOKEN must contain at least 32 characters")
	}
	return result, nil
}

func value(name, fallback string) string {
	if configured := os.Getenv(name); configured != "" {
		return configured
	}
	return fallback
}

func emailSet(value string) map[string]bool {
	result := make(map[string]bool)
	for _, email := range strings.Split(value, ",") {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
			result[email] = true
		}
	}
	return result
}
