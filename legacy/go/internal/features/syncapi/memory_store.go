package syncapi

import (
	"context"
	"sync"
)

type storedOperation struct {
	Cursor   int64
	DeviceID string
	Applied  bool
	Op       syncOperation
}

type MemoryStore struct {
	mu               sync.Mutex
	nextCursor       int64
	latestCursor     int64
	operations       []storedOperation
	idempotencyIndex map[string]int64
	devices          map[string]deviceRegistration
	entities         map[string]entityStateSnapshot
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		idempotencyIndex: map[string]int64{},
		devices:          map[string]deviceRegistration{},
		entities:         map[string]entityStateSnapshot{},
	}
}

func entityMapKey(entityType, entityID string) string {
	return entityType + "::" + entityID
}

func (s *MemoryStore) RegisterDevice(_ context.Context, req deviceRegistration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[req.DeviceID] = req
	return nil
}

func (s *MemoryStore) Push(_ context.Context, deviceID string, operations []syncOperation) (pushResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accepted := make([]string, 0, len(operations))
	conflicts := make([]conflictEvent, 0)

	for _, op := range operations {
		key := deviceID + "::" + op.IdempotencyKey
		if _, exists := s.idempotencyIndex[key]; exists {
			accepted = append(accepted, op.OpID)
			continue
		}

		entityKey := entityMapKey(op.EntityType, op.EntityID)
		current, hasCurrent := s.entities[entityKey]

		isConflict := hasCurrent && op.BaseVersion != current.Version
		incomingWins := true
		if isConflict {
			incomingWins = incomingWinsByLWW(op, deviceID, current)
		}

		applied := !isConflict || incomingWins

		s.nextCursor++
		s.latestCursor = s.nextCursor
		s.idempotencyIndex[key] = s.latestCursor
		s.operations = append(s.operations, storedOperation{Cursor: s.latestCursor, DeviceID: deviceID, Applied: applied, Op: op})

		if applied {
			nextVersion := 1
			if hasCurrent {
				nextVersion = current.Version + 1
			}
			s.entities[entityKey] = entityStateSnapshot{
				Version:           nextVersion,
				LastSeq:           s.latestCursor,
				LastOpID:          op.OpID,
				LastDeviceID:      deviceID,
				LastClientUpdated: op.ClientUpdated,
			}
		}

		if isConflict {
			event := conflictEvent{
				EntityType: op.EntityType,
				EntityID:   op.EntityID,
				Reason:     "base_version_mismatch",
			}

			if incomingWins {
				event.Winner = op.OpID
				event.Loser = current.LastOpID
				event.WinnerDeviceID = deviceID
				event.LoserDeviceID = current.LastDeviceID
			} else {
				event.Winner = current.LastOpID
				event.Loser = op.OpID
				event.WinnerDeviceID = current.LastDeviceID
				event.LoserDeviceID = deviceID
			}

			conflicts = append(conflicts, event)
		}

		accepted = append(accepted, op.OpID)
	}

	return pushResult{
		Accepted:     accepted,
		Conflicts:    conflicts,
		ServerCursor: s.latestCursor,
	}, nil
}

func (s *MemoryStore) Pull(_ context.Context, cursor int64, limit int) ([]syncOperation, int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 100
	}

	out := make([]syncOperation, 0, limit)
	var nextCursor int64 = cursor
	hasMore := false

	for _, item := range s.operations {
		if item.Cursor <= cursor {
			continue
		}
		if !item.Applied {
			continue
		}
		if len(out) >= limit {
			hasMore = true
			break
		}
		out = append(out, item.Op)
		nextCursor = item.Cursor
	}

	return out, nextCursor, hasMore, nil
}

func (s *MemoryStore) LatestCursor(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latestCursor, nil
}
