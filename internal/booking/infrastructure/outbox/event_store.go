package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/example/coworking/internal/booking/application"
	"github.com/example/coworking/internal/booking/domain"
)

type OutboxEvent struct {
	ID        uuid.UUID
	EventType string
	EventData json.RawMessage
	Published bool 
	CreatedAt time.Time

	RetryCount  int
	NextRetryAt time.Time
	LastError   string
	PublishedAt *time.Time
}

type EventStore struct {
	mu     sync.RWMutex
	events []OutboxEvent
}

func (e OutboxEvent) EventName() string {
    return e.EventType
}

func NewEventStore(bus application.EventBus) application.EventStore {
	return &EventStore{
		events: make([]OutboxEvent, 0),
	}
}

func (s *EventStore) SaveEvents(ctx context.Context, events []domain.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}

		outboxEvent := OutboxEvent{
			ID:          uuid.New(),
			EventType:   getEventType(event),
			EventData:   data,
			Published:   false,
			CreatedAt:   time.Now(),
			RetryCount:  0,
			NextRetryAt: time.Now(),
			LastError:   "",
			PublishedAt: nil,
		}

		s.events = append(s.events, outboxEvent)
	}
	// TODO: replace goroutine-based publish with a reliable polling publisher.
	// Current approach may lose events if the process crashes before publishing.
	return nil
}

func (s *EventStore) publishPendingEvents(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var domainEvents []domain.Event
	for i, event := range s.events {
		if !event.Published {
			var domainEvent domain.Event
			switch event.EventType {
			case "RoomBooked":
				var e domain.RoomBooked
				if err := json.Unmarshal(event.EventData, &e); err == nil {
					domainEvent = e
				}
			case "BookingConfirmed":
				var e domain.BookingConfirmed
				if err := json.Unmarshal(event.EventData, &e); err == nil {
					domainEvent = e
				}
			}

			if domainEvent != nil {
				domainEvents = append(domainEvents, domainEvent)
				s.events[i].Published = true
			}
		}
	}
}

func getEventType(event domain.Event) string {
	switch event.(type) {
	case domain.RoomBooked:
		return "RoomBooked"
	case domain.BookingConfirmed:
		return "BookingConfirmed"
	default:
		return "Unknown"
	}
}

func (s *EventStore) GetPendingEvents(ctx context.Context, batchSize int, now time.Time) []OutboxEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]OutboxEvent, 0, batchSize)
	for _, event := range s.events {
		if !event.Published {
			if event.NextRetryAt.Before(now) || event.NextRetryAt.Equal(now) {
				result = append(result, event)
				if len(result) >= batchSize {
					return result
				}
			}
		}
	}
	return result
}


func (s *EventStore) MarkPublished(ctx context.Context, id uuid.UUID, publishedAt time.Time) error{
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.events{
		if s.events[i].ID == id{
			s.events[i].Published = true
			s.events[i].PublishedAt = &publishedAt
			s.events[i].LastError = ""
			return nil
		}
	}
	return fmt.Errorf("outbox event %s not found", id)
}

func (s *EventStore) ScheduleRetry(ctx context.Context, id uuid.UUID, nextRetry time.Time, lastErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.events{
		if s.events[i].ID == id{
			s.events[i].RetryCount++
			s.events[i].NextRetryAt = nextRetry
			if lastErr != nil{
				s.events[i].LastError = lastErr.Error()
			}
			return nil
		}
	}
	return fmt.Errorf("outbox event %s not found", id)
}