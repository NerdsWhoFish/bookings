package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const sessionCookie = "bookings_admin"

type Session struct {
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expiresAt"`
}

type Manager struct {
	key    []byte
	secure bool
}

func NewManager(key string, secure bool) *Manager {
	return &Manager{key: []byte(key), secure: secure}
}

func (m *Manager) Issue(response http.ResponseWriter, email string) error {
	encoded, err := m.sign(Session{Email: strings.ToLower(email), ExpiresAt: time.Now().Add(12 * time.Hour).Unix()})
	if err != nil {
		return err
	}
	http.SetCookie(response, &http.Cookie{
		Name: sessionCookie, Value: encoded, Path: "/", MaxAge: 12 * 60 * 60,
		HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (m *Manager) Read(request *http.Request) (Session, error) {
	cookie, err := request.Cookie(sessionCookie)
	if err != nil {
		return Session{}, err
	}
	var session Session
	if err := m.verify(cookie.Value, &session); err != nil {
		return Session{}, err
	}
	if session.ExpiresAt < time.Now().Unix() {
		return Session{}, errors.New("session expired")
	}
	return session, nil
}

func (m *Manager) NewState(response http.ResponseWriter) (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(value)
	signed, err := m.sign(struct {
		State     string `json:"state"`
		ExpiresAt int64  `json:"expiresAt"`
	}{State: state, ExpiresAt: time.Now().Add(10 * time.Minute).Unix()})
	if err != nil {
		return "", err
	}
	http.SetCookie(response, &http.Cookie{
		Name: "bookings_oauth_state", Value: signed, Path: "/api/admin/google/callback", MaxAge: 600,
		HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode,
	})
	return state, nil
}

func (m *Manager) VerifyState(request *http.Request, expected string) error {
	cookie, err := request.Cookie("bookings_oauth_state")
	if err != nil {
		return err
	}
	var state struct {
		State     string `json:"state"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	if err := m.verify(cookie.Value, &state); err != nil {
		return err
	}
	if state.ExpiresAt < time.Now().Unix() || subtle.ConstantTimeCompare([]byte(state.State), []byte(expected)) != 1 {
		return errors.New("invalid OAuth state")
	}
	return nil
}

func (m *Manager) sign(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (m *Manager) verify(value string, target any) error {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return errors.New("invalid signed value")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return errors.New("invalid signature")
	}
	return json.Unmarshal(payload, target)
}
