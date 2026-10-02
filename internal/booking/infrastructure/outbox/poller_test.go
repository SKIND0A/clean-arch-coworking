package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/coworking/internal/booking/domain"
)

type mockEventBus struct {
	errToReturn error
	published   []domain.Event
}

func (m *mockEventBus) Publish(ctx context.Context, events []domain.Event) error {
	m.published = append(m.published, events...)
	return m.errToReturn
}

func TestSuccessfulEventProcessing(t *testing.T) {
	ctx := context.Background()
	bus := &mockEventBus{errToReturn: nil}

	store := NewEventStore(bus).(*EventStore)
	backoff := NewBackoff()

	event := domain.RoomBooked{
		BookingID: "123",
		RoomID:    "1",
		UserID:    "42",
	}

	err := store.SaveEvents(ctx, []domain.Event{event})
	if err != nil {
		t.Fatalf("event saving error %v", err)
	}

	poller := NewOutboxPoller(store, bus, backoff, 1*time.Second, 10)

	err = poller.processBatch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bus.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(bus.published))
	}
	pending := store.GetPendingEvents(ctx, 10, time.Now())
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending events, got %d", len(pending))
	}
}

func TestFailProcessing(t *testing.T) {
	ctx := context.Background()

	busErr := errors.New("network problems")
	bus := &mockEventBus{errToReturn: busErr}

	store := NewEventStore(bus).(*EventStore)
	backoff := NewBackoff()

	event := domain.RoomBooked{
		BookingID: "fail",
		RoomID:    "123",
		UserID:    "321",
	}

	err := store.SaveEvents(ctx, []domain.Event{event})
	if err != nil {
		t.Fatalf("event saving error %v", err)
	}

	poller := NewOutboxPoller(store, bus, backoff, 1*time.Second, 10)

	err = poller.processBatch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bus.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(bus.published))
	}
	pending := store.GetPendingEvents(ctx, 10, time.Now())
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending events, got %d", len(pending))
	}
}
