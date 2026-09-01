package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/config"
	"github.com/NerdsWhoFish/bookings/internal/domain"
	"github.com/NerdsWhoFish/bookings/internal/securetoken"
	"github.com/NerdsWhoFish/bookings/internal/store"
)

func TestValidateCalendarInvitationRejectsInvalidExpiredAndUsedTokens(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	invitation := domain.CalendarInvitation{TokenHash: securetoken.Hash("secret"), ExpiresAt: now.Add(time.Hour)}
	if err := validateCalendarInvitation(invitation, "secret", now); err != nil {
		t.Fatal(err)
	}
	if err := validateCalendarInvitation(invitation, "wrong", now); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected an invalid token to look missing, got %v", err)
	}
	invitation.ExpiresAt = now
	if err := validateCalendarInvitation(invitation, "secret", now); !errors.Is(err, store.ErrExpired) {
		t.Fatalf("expected expired invitation, got %v", err)
	}
	invitation.ExpiresAt = now.Add(time.Hour)
	invitation.UsedAt = &now
	if err := validateCalendarInvitation(invitation, "secret", now); !errors.Is(err, store.ErrAlreadyUsed) {
		t.Fatalf("expected used invitation, got %v", err)
	}
}

func TestExternalBlockAPIRequiresBearerTokenAndUpsertsIdempotently(t *testing.T) {
	data := store.NewMemory(nil)
	server := &Server{
		config: config.Config{ExternalBlockToken: "01234567890123456789012345678901"},
		store:  data,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	handler := server.external(server.putExternalBlock)
	body := `{"start":"2026-09-02T13:00:00Z","end":"2026-09-02T14:00:00Z"}`

	request := httptest.NewRequest(http.MethodPut, "/api/external/blocks/work:event-1", strings.NewReader(body))
	request.SetPathValue("id", "work:event-1")
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("unexpected unauthenticated response: %d, %s", response.Code, response.Body)
	}

	for range 2 {
		request = httptest.NewRequest(http.MethodPut, "/api/external/blocks/work:event-1", strings.NewReader(body))
		request.SetPathValue("id", "work:event-1")
		request.Header.Set("Authorization", "Bearer "+server.config.ExternalBlockToken)
		response = httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("unexpected authenticated response: %d, %s", response.Code, response.Body)
		}
	}
	blocks, err := data.ListExternalBlocks(request.Context(), time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), time.Date(2026, 9, 2, 15, 0, 0, 0, time.UTC))
	if err != nil || len(blocks) != 1 {
		t.Fatalf("expected one idempotent block, got %#v, %v", blocks, err)
	}
}
