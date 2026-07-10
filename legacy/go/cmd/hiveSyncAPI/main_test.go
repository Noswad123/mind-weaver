package main

import (
	"reflect"
	"testing"
)

func TestParseBoolEnv(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "true literal", input: "true", want: true},
		{name: "numeric true", input: "1", want: true},
		{name: "yes", input: "yes", want: true},
		{name: "off", input: "off", want: false},
		{name: "empty", input: "", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseBoolEnv(tc.input); got != tc.want {
				t.Fatalf("parseBoolEnv(%q)=%v want=%v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseDeviceTokenMap_AcceptsEqualsAndColonSeparators(t *testing.T) {
	t.Parallel()

	parsed, err := parseDeviceTokenMap("desktop=abc123, phone:def456")
	if err != nil {
		t.Fatalf("parseDeviceTokenMap error: %v", err)
	}

	if parsed["desktop"] != "abc123" {
		t.Fatalf("desktop token=%q want=%q", parsed["desktop"], "abc123")
	}
	if parsed["phone"] != "def456" {
		t.Fatalf("phone token=%q want=%q", parsed["phone"], "def456")
	}
}

func TestParseDeviceTokenMap_RejectsTokenReuseAcrossDevices(t *testing.T) {
	t.Parallel()

	_, err := parseDeviceTokenMap("desktop=same,phone=same")
	if err == nil {
		t.Fatalf("expected token reuse parse error")
	}
}

func TestParseDeviceTokenMap_RejectsMalformedEntry(t *testing.T) {
	t.Parallel()

	_, err := parseDeviceTokenMap("desktop")
	if err == nil {
		t.Fatalf("expected malformed entry parse error")
	}
}

func TestParseStringList_DedupesAndTrims(t *testing.T) {
	t.Parallel()

	got := parseStringList(" https://a.example,https://b.example ; https://a.example\nhttps://c.example ")
	want := []string{"https://a.example", "https://b.example", "https://c.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseStringList=%v want=%v", got, want)
	}
}

func TestBuildCORSConfigFromEnv(t *testing.T) {
	t.Setenv("HIVE_SYNC_CORS_ALLOWED_ORIGINS", "https://a.example,https://b.example")

	cfg := buildCORSConfigFromEnv()
	if cfg == nil {
		t.Fatalf("expected cors config")
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, []string{"https://a.example", "https://b.example"}) {
		t.Fatalf("allowed origins=%v", cfg.AllowedOrigins)
	}
}
