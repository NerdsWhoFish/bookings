package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

func TestHiddenMeetingIsDirectlyBookableButNotListed(t *testing.T) {
	data := NewMemory([]domain.MeetingType{{ID: "private", Slug: "private", Active: true, Hidden: true}})
	meetings, err := data.ListMeetingTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(meetings) != 0 {
		t.Fatalf("hidden meeting appeared in catalog: %#v", meetings)
	}
	if _, err := data.GetMeetingType(context.Background(), "private"); err != nil {
		t.Fatalf("hidden meeting was not directly bookable: %v", err)
	}
}

func TestDeleteMeetingTypeRemovesItsDirectRoute(t *testing.T) {
	data := NewMemory([]domain.MeetingType{{ID: "quick", Slug: "quick", Active: true}})
	if err := data.DeleteMeetingType(context.Background(), "quick"); err != nil {
		t.Fatal(err)
	}
	if _, err := data.GetMeetingType(context.Background(), "quick"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected deleted meeting to be missing, got %v", err)
	}
	meeting, err := data.GetMeetingTypeRecord(context.Background(), "quick")
	if err != nil || !meeting.Deleted {
		t.Fatalf("expected a cancellation-safe tombstone, got %#v, %v", meeting, err)
	}
}

func TestCalendarInvitationIsEmailBoundExpiringAndOneTime(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	data := NewMemory(nil)
	invitation := domain.CalendarInvitation{ID: "invite", Email: "person@example.com", ExpiresAt: now.Add(time.Hour)}
	if err := data.PutCalendarInvitation(context.Background(), invitation); err != nil {
		t.Fatal(err)
	}
	if err := data.UseCalendarInvitation(context.Background(), invitation.ID, "wrong@example.com", now); !errors.Is(err, ErrEmailMismatch) {
		t.Fatalf("expected email mismatch, got %v", err)
	}
	if err := data.UseCalendarInvitation(context.Background(), invitation.ID, "PERSON@example.com", now); err != nil {
		t.Fatal(err)
	}
	if err := data.UseCalendarInvitation(context.Background(), invitation.ID, invitation.Email, now); !errors.Is(err, ErrAlreadyUsed) {
		t.Fatalf("expected one-time invitation rejection, got %v", err)
	}

	expired := domain.CalendarInvitation{ID: "expired", Email: invitation.Email, ExpiresAt: now}
	if err := data.PutCalendarInvitation(context.Background(), expired); err != nil {
		t.Fatal(err)
	}
	if err := data.UseCalendarInvitation(context.Background(), expired.ID, expired.Email, now); !errors.Is(err, ErrExpired) {
		t.Fatalf("expected expired invitation rejection, got %v", err)
	}
}
