package syncapi

import (
	"context"
	"testing"
)

func TestTokenAuthenticator_DeviceLookup(t *testing.T) {
	t.Parallel()

	a := NewTokenAuthenticator(map[string]string{
		"desktop": "token-desktop",
	})

	deviceID, ok := a.DeviceIDForBearerToken("token-desktop")
	if !ok {
		t.Fatalf("expected token lookup success")
	}
	if deviceID != "desktop" {
		t.Fatalf("device id=%q want=%q", deviceID, "desktop")
	}
}

func TestTokenAuthenticator_UnknownToken(t *testing.T) {
	t.Parallel()

	a := NewTokenAuthenticator(map[string]string{"desktop": "token-desktop"})
	if _, ok := a.DeviceIDForBearerToken("nope"); ok {
		t.Fatalf("expected unknown token lookup to fail")
	}
}

func TestAuthenticatedDeviceIDContextRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := withAuthenticatedDeviceID(context.Background(), "desktop")
	deviceID, ok := authenticatedDeviceIDFromContext(ctx)
	if !ok {
		t.Fatalf("expected device id in context")
	}
	if deviceID != "desktop" {
		t.Fatalf("device id=%q want=%q", deviceID, "desktop")
	}
}
