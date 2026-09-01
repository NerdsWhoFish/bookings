package httpapi

import (
	"errors"
	"testing"
	"time"

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
