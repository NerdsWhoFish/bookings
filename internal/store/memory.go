package store

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

type Memory struct {
	mu             sync.RWMutex
	meetings       map[string]domain.MeetingType
	connections    map[string]domain.CalendarConnection
	invitations    map[string]domain.CalendarInvitation
	externalBlocks map[string]domain.ExternalBlock
	bookings       map[string]domain.Booking
	locks          map[string]string
}

func NewMemory(meetings []domain.MeetingType) *Memory {
	result := &Memory{
		meetings:       make(map[string]domain.MeetingType),
		connections:    make(map[string]domain.CalendarConnection),
		invitations:    make(map[string]domain.CalendarInvitation),
		externalBlocks: make(map[string]domain.ExternalBlock),
		bookings:       make(map[string]domain.Booking),
		locks:          make(map[string]string),
	}
	for _, meeting := range meetings {
		result.meetings[meeting.Slug] = meeting
	}
	return result
}

func (m *Memory) ListMeetingTypes(context.Context) ([]domain.MeetingType, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.MeetingType, 0, len(m.meetings))
	for _, meeting := range m.meetings {
		if meeting.Active && !meeting.Hidden && !meeting.Deleted {
			result = append(result, meeting)
		}
	}
	return result, nil
}

func (m *Memory) ListAllMeetingTypes(context.Context) ([]domain.MeetingType, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.MeetingType, 0, len(m.meetings))
	for _, meeting := range m.meetings {
		if !meeting.Deleted {
			result = append(result, meeting)
		}
	}
	return result, nil
}

func (m *Memory) GetMeetingType(_ context.Context, slug string) (domain.MeetingType, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	meeting, ok := m.meetings[slug]
	if !ok {
		for _, candidate := range m.meetings {
			if candidate.ID == slug {
				meeting, ok = candidate, true
				break
			}
		}
	}
	if !ok || !meeting.Active || meeting.Deleted {
		return domain.MeetingType{}, ErrNotFound
	}
	return meeting, nil
}

func (m *Memory) GetMeetingTypeRecord(_ context.Context, id string) (domain.MeetingType, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, meeting := range m.meetings {
		if meeting.ID == id {
			return meeting, nil
		}
	}
	return domain.MeetingType{}, ErrNotFound
}

func (m *Memory) PutMeetingType(_ context.Context, meeting domain.MeetingType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for slug, existing := range m.meetings {
		if existing.ID == meeting.ID && slug != meeting.Slug {
			delete(m.meetings, slug)
		}
	}
	m.meetings[meeting.Slug] = meeting
	return nil
}

func (m *Memory) DeleteMeetingType(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for slug, meeting := range m.meetings {
		if meeting.ID == id {
			meeting.Deleted = true
			m.meetings[slug] = meeting
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) ListConnections(context.Context) ([]domain.CalendarConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.CalendarConnection, 0, len(m.connections))
	for _, connection := range m.connections {
		result = append(result, connection)
	}
	return result, nil
}

func (m *Memory) GetConnection(_ context.Context, id string) (domain.CalendarConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connection, ok := m.connections[id]
	if !ok {
		return domain.CalendarConnection{}, ErrNotFound
	}
	return connection, nil
}

func (m *Memory) PutConnection(_ context.Context, connection domain.CalendarConnection) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections[connection.ID] = connection
	return nil
}

func (m *Memory) ListCalendarInvitations(context.Context) ([]domain.CalendarInvitation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.CalendarInvitation, 0, len(m.invitations))
	for _, invitation := range m.invitations {
		result = append(result, invitation)
	}
	return result, nil
}

func (m *Memory) GetCalendarInvitation(_ context.Context, id string) (domain.CalendarInvitation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	invitation, ok := m.invitations[id]
	if !ok {
		return domain.CalendarInvitation{}, ErrNotFound
	}
	return invitation, nil
}

func (m *Memory) PutCalendarInvitation(_ context.Context, invitation domain.CalendarInvitation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invitations[invitation.ID] = invitation
	return nil
}

func (m *Memory) DeleteCalendarInvitation(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.invitations[id]; !ok {
		return ErrNotFound
	}
	delete(m.invitations, id)
	return nil
}

func (m *Memory) UseCalendarInvitation(_ context.Context, id, email string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	invitation, ok := m.invitations[id]
	if !ok {
		return ErrNotFound
	}
	if invitation.UsedAt != nil {
		return ErrAlreadyUsed
	}
	if !now.Before(invitation.ExpiresAt) {
		return ErrExpired
	}
	if !strings.EqualFold(invitation.Email, email) {
		return ErrEmailMismatch
	}
	usedAt := now.UTC()
	invitation.UsedAt = &usedAt
	m.invitations[id] = invitation
	return nil
}

func (m *Memory) ListExternalBlocks(_ context.Context, start, end time.Time) ([]domain.ExternalBlock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.ExternalBlock, 0)
	for _, block := range m.externalBlocks {
		if block.Start.Before(end) && block.End.After(start) {
			result = append(result, block)
		}
	}
	return result, nil
}

func (m *Memory) PutExternalBlock(_ context.Context, block domain.ExternalBlock) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.externalBlocks[block.ID]; ok {
		block.CreatedAt = existing.CreatedAt
	}
	m.externalBlocks[block.ID] = block
	return nil
}

func (m *Memory) DeleteExternalBlock(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.externalBlocks, id)
	return nil
}

func (m *Memory) ClaimBooking(_ context.Context, booking domain.Booking) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, lockID := range booking.LockIDs {
		if _, exists := m.locks[lockID]; exists {
			return ErrConflict
		}
	}
	for _, lockID := range booking.LockIDs {
		m.locks[lockID] = booking.ID
	}
	m.bookings[booking.ID] = booking
	return nil
}

func (m *Memory) ConfirmBooking(_ context.Context, id, eventID string, shadowEventIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	booking, ok := m.bookings[id]
	if !ok {
		return ErrNotFound
	}
	booking.Status = "confirmed"
	booking.EventID = eventID
	booking.ShadowEventIDs = append([]string(nil), shadowEventIDs...)
	booking.UpdatedAt = time.Now().UTC()
	m.bookings[id] = booking
	return nil
}

func (m *Memory) FailBooking(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	booking, ok := m.bookings[id]
	if !ok {
		return ErrNotFound
	}
	for _, lockID := range booking.LockIDs {
		delete(m.locks, lockID)
	}
	booking.Status = "failed"
	m.bookings[id] = booking
	return nil
}

func (m *Memory) GetBooking(_ context.Context, id string) (domain.Booking, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	booking, ok := m.bookings[id]
	if !ok {
		return domain.Booking{}, ErrNotFound
	}
	return booking, nil
}

func (m *Memory) CancelBooking(_ context.Context, id string) (domain.Booking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	booking, ok := m.bookings[id]
	if !ok {
		return domain.Booking{}, ErrNotFound
	}
	for _, lockID := range booking.LockIDs {
		delete(m.locks, lockID)
	}
	booking.Status = "canceled"
	booking.UpdatedAt = time.Now().UTC()
	m.bookings[id] = booking
	return booking, nil
}
