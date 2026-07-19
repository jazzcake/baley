package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestOpenRequiresStableRunLeaseSecret(t *testing.T) {
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "")
	_, err := Open(context.Background(), "postgres://unused")
	if err == nil || !strings.Contains(err.Error(), "BALEY_LEASE_TOKEN_SECRET is required") {
		t.Fatalf("missing lease secret error = %v", err)
	}
}
