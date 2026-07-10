package syncapi

import "context"

type deviceRegistration struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

type pushResult struct {
	Accepted     []string
	Conflicts    []conflictEvent
	ServerCursor int64
}

type Store interface {
	RegisterDevice(ctx context.Context, req deviceRegistration) error
	Push(ctx context.Context, deviceID string, operations []syncOperation) (pushResult, error)
	Pull(ctx context.Context, cursor int64, limit int) ([]syncOperation, int64, bool, error)
	LatestCursor(ctx context.Context) (int64, error)
}
