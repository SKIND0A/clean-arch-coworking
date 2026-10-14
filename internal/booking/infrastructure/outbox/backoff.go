package outbox

import (
	"math"
	"time"
)

const(
	FirstInterval = 1 * time.Second
	MultiplierAttempts = 2.0
	LastInterval = 1 * time.Minute
	LimitAttemps = 5
)

type Backoff struct {
	InitialInterval time.Duration
	Multiplier float64
	MaxInterval time.Duration
	MaxRetries int
}

func NewBackoff() *Backoff {
    return &Backoff{
        InitialInterval: FirstInterval,
        Multiplier:      MultiplierAttempts,
        MaxInterval:      LastInterval,
        MaxRetries:       LimitAttemps,
    }
}

func (b *Backoff) NextDelay(count int) time.Duration{

	factor := math.Pow(b.Multiplier, float64(count))

	delay := time.Duration(float64(b.InitialInterval) * factor)

	if delay > b.MaxInterval{
		return b.MaxInterval
	}
	return delay
}

func (b *Backoff) IsExceeded(retryCount int) bool{
	return retryCount > b.MaxRetries
}