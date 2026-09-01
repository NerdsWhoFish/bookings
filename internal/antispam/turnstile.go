package antispam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Verifier interface {
	Verify(context.Context, string, string) error
}

type Disabled struct{}

func (Disabled) Verify(context.Context, string, string) error { return nil }

type Turnstile struct {
	secret string
	client *http.Client
}

func NewTurnstile(secret string, client *http.Client) *Turnstile {
	return &Turnstile{secret: secret, client: client}
}

func (t *Turnstile) Verify(ctx context.Context, token, remoteIP string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(url.Values{
		"secret":   {t.secret},
		"response": {token},
		"remoteip": {remoteIP},
	}.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := t.client.Do(request)
	if err != nil {
		return fmt.Errorf("verify Turnstile challenge: %w", err)
	}
	defer response.Body.Close()
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode Turnstile response: %w", err)
	}
	if !result.Success {
		return errors.New("challenge rejected")
	}
	return nil
}
