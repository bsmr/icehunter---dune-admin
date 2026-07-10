package main

import "testing"

// TestShutdownPayload is a regression test for #262 sub-bug A: the in-game
// countdown never rendered because DateTimestamp and ShutdownTimestamp were
// set to the same value (the client derives the countdown window as
// ShutdownTimestamp - DateTimestamp, which came out to 0), ShutdownDuration
// carried an unrelated form value instead of the lead time, and
// BroadcastDuration (the on-screen pulse length) was omitted entirely — even
// though the working Generic broadcast already sets it correctly.
func TestShutdownPayload(t *testing.T) {
	t.Parallel()

	const now int64 = 1_000_000
	tests := []struct {
		name              string
		timestamp         int64
		wantShutdownDur   int
		wantDateTimestamp int64
	}{
		{
			name:              "future shutdown: duration is the lead time",
			timestamp:         now + 300,
			wantShutdownDur:   300,
			wantDateTimestamp: now,
		},
		{
			name:              "shutdown at now: zero lead",
			timestamp:         now,
			wantShutdownDur:   0,
			wantDateTimestamp: now,
		},
		{
			name:              "timestamp before now (e.g. a cancel): clamped to zero, never negative",
			timestamp:         now - 60,
			wantShutdownDur:   0,
			wantDateTimestamp: now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload := shutdownPayload(now, tt.timestamp, "Restart", 60, 30, false)

			if got := payload["DateTimestamp"]; got != tt.wantDateTimestamp {
				t.Errorf("DateTimestamp = %v, want %v", got, tt.wantDateTimestamp)
			}
			if got := payload["ShutdownTimestamp"]; got != tt.timestamp {
				t.Errorf("ShutdownTimestamp = %v, want %v", got, tt.timestamp)
			}
			if got := payload["ShutdownDuration"]; got != tt.wantShutdownDur {
				t.Errorf("ShutdownDuration = %v, want %v", got, tt.wantShutdownDur)
			}
			if got := payload["BroadcastDuration"]; got != 30 {
				t.Errorf("BroadcastDuration = %v, want 30", got)
			}
			if got := payload["BroadcastFrequency"]; got != 60 {
				t.Errorf("BroadcastFrequency = %v, want 60", got)
			}
			if got := payload["ShutdownType"]; got != "Restart" {
				t.Errorf("ShutdownType = %v, want Restart", got)
			}
			if got := payload["ShouldCancel"]; got != false {
				t.Errorf("ShouldCancel = %v, want false", got)
			}
		})
	}
}
