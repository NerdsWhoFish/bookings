package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NerdsWhoFish/bookings/internal/auth"
	"github.com/NerdsWhoFish/bookings/internal/config"
	"github.com/NerdsWhoFish/bookings/internal/domain"
	"github.com/NerdsWhoFish/bookings/internal/store"
)

func TestAdminSessionProbeWithoutCookie(t *testing.T) {
	var logs bytes.Buffer
	sessions := auth.NewManager("test-session-signing-key", false)
	handler := New(config.Config{}, nil, nil, nil, nil, sessions, nil, nil, slog.New(slog.NewJSONHandler(&logs, nil)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/admin/session", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", response.Code, response.Body)
	}
	var problem struct {
		Title  string `json:"title"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil || problem.Title != "Sign in required" || problem.Status != 401 {
		t.Fatalf("expected sign-in problem response, got %s (%v)", response.Body, err)
	}
	if strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("ordinary signed-out session probe logged a server error: %s", &logs)
	}
	if !strings.Contains(logs.String(), `"http.response.status_code":401`) || !strings.Contains(logs.String(), `"http.route":"GET /api/admin/session"`) {
		t.Fatalf("expected observable 401 request completion, got %s", &logs)
	}
}

func TestAdminAuthenticationStillRejectsAndReportsUnexpectedRequests(t *testing.T) {
	for _, tc := range []struct {
		name, method, route string
		cookie              *http.Cookie
	}{
		{name: "invalid session", method: http.MethodGet, route: "/api/admin/session", cookie: &http.Cookie{Name: "bookings_admin", Value: "invalid"}},
		{name: "connections", method: http.MethodGet, route: "/api/admin/connections"},
		{name: "meeting types", method: http.MethodGet, route: "/api/admin/meeting-types"},
		{name: "invitations", method: http.MethodGet, route: "/api/admin/calendar-invitations"},
		{name: "mutation", method: http.MethodPut, route: "/api/admin/meeting-types/test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			sessions := auth.NewManager("test-session-signing-key", false)
			handler := New(config.Config{}, nil, nil, nil, nil, sessions, nil, nil, slog.New(slog.NewJSONHandler(&logs, nil)))
			request := httptest.NewRequest(tc.method, tc.route, nil)
			if tc.cookie != nil {
				request.AddCookie(tc.cookie)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized || !strings.Contains(logs.String(), `"level":"ERROR"`) {
				t.Fatalf("expected rejected and reported request, got %d: %s; logs: %s", response.Code, response.Body, &logs)
			}
		})
	}
}

type unavailableConnections struct{ store.Store }

func (unavailableConnections) ListConnections(context.Context) ([]domain.CalendarConnection, error) {
	return nil, errors.New("store unavailable")
}

func TestAuthenticatedAdminSessionAndFailures(t *testing.T) {
	var logs bytes.Buffer
	sessions := auth.NewManager("test-session-signing-key", false)
	issued := httptest.NewRecorder()
	if err := sessions.Issue(issued, "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	cookie := issued.Result().Cookies()[0]
	handler := New(config.Config{PublicURL: "https://book.example.com"}, unavailableConnections{}, nil, nil, nil, sessions, nil, nil, slog.New(slog.NewJSONHandler(&logs, nil)))
	for _, tc := range []struct {
		method, route string
		status        int
	}{
		{http.MethodGet, "/api/admin/session", http.StatusOK},
		{http.MethodGet, "/api/admin/connections", http.StatusInternalServerError},
		{http.MethodPut, "/api/admin/meeting-types/test", http.StatusForbidden},
	} {
		logs.Reset()
		request := httptest.NewRequest(tc.method, tc.route, nil)
		request.AddCookie(cookie)
		request.Header.Set("Origin", "https://other.example.com")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s: expected %d, got %d: %s", tc.route, tc.status, response.Code, response.Body)
		}
		if tc.status == http.StatusOK {
			var session auth.Session
			if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil || session.Email != "admin@example.com" {
				t.Fatalf("expected authenticated session, got %s (%v)", response.Body, err)
			}
		} else if !strings.Contains(logs.String(), `"level":"ERROR"`) {
			t.Fatalf("%s: failure no longer logged: %s", tc.route, &logs)
		}
	}
}
