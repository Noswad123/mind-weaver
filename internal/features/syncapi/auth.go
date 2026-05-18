package syncapi

import (
	"context"
	"strings"
)

type TokenAuthenticator struct {
	deviceByToken map[string]string
}

func NewTokenAuthenticator(deviceTokens map[string]string) *TokenAuthenticator {
	if len(deviceTokens) == 0 {
		return nil
	}

	deviceByToken := make(map[string]string, len(deviceTokens))
	for deviceID, token := range deviceTokens {
		d := strings.TrimSpace(deviceID)
		t := strings.TrimSpace(token)
		if d == "" || t == "" {
			continue
		}
		deviceByToken[t] = d
	}

	if len(deviceByToken) == 0 {
		return nil
	}

	return &TokenAuthenticator{deviceByToken: deviceByToken}
}

func (a *TokenAuthenticator) DeviceIDForBearerToken(token string) (string, bool) {
	if a == nil {
		return "", false
	}

	deviceID, ok := a.deviceByToken[strings.TrimSpace(token)]
	if !ok || strings.TrimSpace(deviceID) == "" {
		return "", false
	}

	return deviceID, true
}

type authDeviceIDContextKey struct{}

func withAuthenticatedDeviceID(ctx context.Context, deviceID string) context.Context {
	return context.WithValue(ctx, authDeviceIDContextKey{}, deviceID)
}

func authenticatedDeviceIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(authDeviceIDContextKey{}).(string)
	if !ok {
		return "", false
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}

	return trimmed, true
}
