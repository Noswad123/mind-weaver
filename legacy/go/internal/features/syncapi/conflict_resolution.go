package syncapi

import (
	"strings"
	"time"
)

type entityStateSnapshot struct {
	Version           int
	LastSeq           int64
	LastOpID          string
	LastDeviceID      string
	LastClientUpdated string
}

func parseRFC3339(value string) (time.Time, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func incomingWinsByLWW(incoming syncOperation, incomingDeviceID string, current entityStateSnapshot) bool {
	incomingTime, incomingOK := parseRFC3339(incoming.ClientUpdated)
	currentTime, currentOK := parseRFC3339(current.LastClientUpdated)

	if incomingOK && currentOK {
		if incomingTime.After(currentTime) {
			return true
		}
		if incomingTime.Before(currentTime) {
			return false
		}
	}

	if incomingOK && !currentOK {
		return true
	}
	if !incomingOK && currentOK {
		return false
	}

	if cmp := strings.Compare(incoming.OpID, current.LastOpID); cmp != 0 {
		return cmp > 0
	}

	return strings.Compare(strings.TrimSpace(incomingDeviceID), strings.TrimSpace(current.LastDeviceID)) >= 0
}
