package store

import (
	"context"
	"sync"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

type Memory struct {
	mu          sync.RWMutex
	meetings    map[string]domain.MeetingType
	connections map[string]domain.CalendarConnection
	bookings    map[string]domain.Booking
	locks       map[string]string
}

func NewMemory(meetings []domain.MeetingType) *Memory {
	result := &Memory{
		meetings:    make(map[string]domain.MeetingType),
		connections: make(map[string]domain.CalendarConnection),
		bookings:    make(map[string]domain.Booking),
		locks:       make(map[string]string),
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
		if meeting.Active {
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
	if !ok || !meeting.Active {
		return domain.MeetingType{}, ErrNotFound
	}
	return meeting, nil
}

func (m *Memory) PutMeetingType(_ context.Context, meeting domain.MeetingType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.meetings[meeting.Slug] = meeting
	return nil
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

func (m *Memory) ConfirmBooking(_ context.Context, id, eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	booking, ok := m.bookings[id]
	if !ok {
		return ErrNotFound
	}
	booking.Status = "confirmed"
	booking.EventID = eventID
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
