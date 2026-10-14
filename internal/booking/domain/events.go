package domain

import (
	"time"

	"github.com/google/uuid"
)

type Event interface{}

type RoomBooked struct {
	BookingID string
	RoomID    string
	UserID    string
}

type BookingConfirmed struct {
	BookingID string
	TxID      string
}

type BookingCancelled struct {
	BookingID uuid.UUID
	CancellAt time.Time
}
