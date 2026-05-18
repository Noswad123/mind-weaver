package syncops

import "time"

type ConflictEvent struct {
	EntityType     EntityType
	EntityKey      string
	Reason         string
	WinnerDeviceID string
	LoserDeviceID  string
	ServerCursor   string
	LocalPayload   string
	RemotePayload  string
	OccurredAt     time.Time
}
