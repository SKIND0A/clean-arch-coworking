package outbox

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/example/coworking/internal/booking/application"
	"github.com/example/coworking/internal/booking/domain"
)

type OutboxPoller struct {
	wg        sync.WaitGroup
	stopChan  chan struct{}
	store     *EventStore
	backoff   *Backoff
	bus       application.EventBus
	interval  time.Duration
	batchSize int
}

func NewOutboxPoller(
	store *EventStore,
	bus application.EventBus,
	backoff *Backoff,
	interval time.Duration,
	batchSize int,
) *OutboxPoller {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 20
	}

	return &OutboxPoller{
		store:     store,
		bus:       bus,
		backoff:   backoff,
		interval:  interval,
		batchSize: batchSize,
		stopChan:  make(chan struct{}),
	}
}

func (p *OutboxPoller) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		clock := time.NewTicker(p.interval)
		defer clock.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopChan:
				return
			case <-clock.C:
				if err := p.processBatch(ctx); err != nil {
					log.Printf("processing error outbox: %v", err)
				}
			}
		}

	}()
}

func (p *OutboxPoller) Stop() {
	close(p.stopChan)
	p.wg.Wait()
}

func (p *OutboxPoller) processBatch(ctx context.Context) error {
	events := p.store.GetPendingEvents(ctx, p.batchSize, time.Now())

	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := p.bus.Publish(ctx, []domain.Event{event})
		if err == nil {
			if markErr := p.store.MarkPublished(ctx, event.ID, time.Now()); markErr != nil {
				return markErr
			}
			continue
		}

		if p.backoff.IsExceeded(event.RetryCount) {
			_ = p.store.ScheduleRetry(ctx, event.ID, time.Time{}, err)
			continue
		}

		delay := p.backoff.NextDelay(event.RetryCount)
		nextRetry := time.Now().Add(delay)

		if schedErr := p.store.ScheduleRetry(ctx, event.ID, nextRetry, err); schedErr != nil {
			return schedErr
		}
	}

	return nil
}
