package store

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/NerdsWhoFish/bookings/internal/domain"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Firestore struct {
	client *firestore.Client
}

func NewFirestore(client *firestore.Client) *Firestore {
	return &Firestore{client: client}
}

func (s *Firestore) ListMeetingTypes(ctx context.Context) ([]domain.MeetingType, error) {
	return s.listMeetingTypes(s.client.Collection("meeting_types").Where("active", "==", true).Documents(ctx))
}

func (s *Firestore) ListAllMeetingTypes(ctx context.Context) ([]domain.MeetingType, error) {
	return s.listMeetingTypes(s.client.Collection("meeting_types").Documents(ctx))
}

func (s *Firestore) listMeetingTypes(documents *firestore.DocumentIterator) ([]domain.MeetingType, error) {
	defer documents.Stop()
	result := make([]domain.MeetingType, 0)
	for {
		document, err := documents.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var meeting domain.MeetingType
		if err := document.DataTo(&meeting); err != nil {
			return nil, err
		}
		result = append(result, meeting)
	}
	return result, nil
}

func (s *Firestore) GetMeetingType(ctx context.Context, slug string) (domain.MeetingType, error) {
	if document, err := s.client.Collection("meeting_types").Doc(slug).Get(ctx); err == nil {
		var meeting domain.MeetingType
		if err := document.DataTo(&meeting); err != nil {
			return domain.MeetingType{}, err
		}
		if meeting.Active {
			return meeting, nil
		}
	} else if status.Code(err) != codes.NotFound {
		return domain.MeetingType{}, err
	}
	documents := s.client.Collection("meeting_types").Where("slug", "==", slug).Where("active", "==", true).Limit(1).Documents(ctx)
	defer documents.Stop()
	document, err := documents.Next()
	if errors.Is(err, iterator.Done) {
		return domain.MeetingType{}, ErrNotFound
	}
	if err != nil {
		return domain.MeetingType{}, err
	}
	var meeting domain.MeetingType
	if err := document.DataTo(&meeting); err != nil {
		return domain.MeetingType{}, err
	}
	return meeting, nil
}

func (s *Firestore) PutMeetingType(ctx context.Context, meeting domain.MeetingType) error {
	_, err := s.client.Collection("meeting_types").Doc(meeting.ID).Set(ctx, meeting)
	return err
}

func (s *Firestore) ListConnections(ctx context.Context) ([]domain.CalendarConnection, error) {
	documents := s.client.Collection("calendar_connections").Documents(ctx)
	defer documents.Stop()
	result := make([]domain.CalendarConnection, 0)
	for {
		document, err := documents.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var connection domain.CalendarConnection
		if err := document.DataTo(&connection); err != nil {
			return nil, err
		}
		result = append(result, connection)
	}
	return result, nil
}

func (s *Firestore) GetConnection(ctx context.Context, id string) (domain.CalendarConnection, error) {
	document, err := s.client.Collection("calendar_connections").Doc(id).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return domain.CalendarConnection{}, ErrNotFound
		}
		return domain.CalendarConnection{}, err
	}
	var connection domain.CalendarConnection
	if err := document.DataTo(&connection); err != nil {
		return domain.CalendarConnection{}, err
	}
	return connection, nil
}

func (s *Firestore) PutConnection(ctx context.Context, connection domain.CalendarConnection) error {
	_, err := s.client.Collection("calendar_connections").Doc(connection.ID).Set(ctx, connection)
	return err
}

func (s *Firestore) ClaimBooking(ctx context.Context, booking domain.Booking) error {
	return s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		bookingRef := s.client.Collection("bookings").Doc(booking.ID)
		if _, err := tx.Get(bookingRef); err == nil {
			return ErrConflict
		} else if status.Code(err) != codes.NotFound {
			return err
		}
		for _, id := range booking.LockIDs {
			if _, err := tx.Get(s.client.Collection("slot_locks").Doc(id)); err == nil {
				return ErrConflict
			} else if status.Code(err) != codes.NotFound {
				return err
			}
		}
		if err := tx.Create(bookingRef, booking); err != nil {
			return err
		}
		for _, id := range booking.LockIDs {
			if err := tx.Create(s.client.Collection("slot_locks").Doc(id), map[string]any{
				"booking_id": booking.ID,
				"expires_at": booking.End.Add(24 * time.Hour),
			}); err != nil {
				return err
			}
		}
		return nil
	}, firestore.MaxAttempts(32))
}

func (s *Firestore) ConfirmBooking(ctx context.Context, id, eventID string) error {
	_, err := s.client.Collection("bookings").Doc(id).Update(ctx, []firestore.Update{
		{Path: "status", Value: "confirmed"},
		{Path: "event_id", Value: eventID},
		{Path: "updated_at", Value: time.Now().UTC()},
	})
	return err
}

func (s *Firestore) FailBooking(ctx context.Context, id string) error {
	return s.release(ctx, id, "failed")
}

func (s *Firestore) GetBooking(ctx context.Context, id string) (domain.Booking, error) {
	document, err := s.client.Collection("bookings").Doc(id).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return domain.Booking{}, ErrNotFound
		}
		return domain.Booking{}, err
	}
	var booking domain.Booking
	if err := document.DataTo(&booking); err != nil {
		return domain.Booking{}, err
	}
	return booking, nil
}

func (s *Firestore) CancelBooking(ctx context.Context, id string) (domain.Booking, error) {
	booking, err := s.GetBooking(ctx, id)
	if err != nil {
		return domain.Booking{}, err
	}
	if err := s.release(ctx, id, "canceled"); err != nil {
		return domain.Booking{}, err
	}
	booking.Status = "canceled"
	return booking, nil
}

func (s *Firestore) release(ctx context.Context, id, status string) error {
	return s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		ref := s.client.Collection("bookings").Doc(id)
		document, err := tx.Get(ref)
		if err != nil {
			return err
		}
		var booking domain.Booking
		if err := document.DataTo(&booking); err != nil {
			return err
		}
		if err := tx.Update(ref, []firestore.Update{{Path: "status", Value: status}, {Path: "updated_at", Value: time.Now().UTC()}}); err != nil {
			return err
		}
		for _, lockID := range booking.LockIDs {
			if err := tx.Delete(s.client.Collection("slot_locks").Doc(lockID)); err != nil {
				return err
			}
		}
		return nil
	}, firestore.MaxAttempts(32))
}
