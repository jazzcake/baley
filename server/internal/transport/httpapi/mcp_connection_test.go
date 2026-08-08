package httpapi

import (
	"testing"
	"time"
)

func TestMCPConnectionTTLIsThirtyMinutes(t *testing.T) {
	now := time.Date(2026, 8, 8, 6, 30, 0, 0, time.UTC)
	if got := now.Add(mcpConnectionTTL).Sub(now); got != 30*time.Minute {
		t.Fatalf("approval TTL = %s, want 30m", got)
	}
}
