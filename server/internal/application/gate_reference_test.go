package application

import "testing"

func TestFindGateByReferenceSupportsInternalPublicAndAlias(t *testing.T) {
	gates := []GateProjection{
		{ID: "stable-gate-id", PublicID: 7, Alias: "release-ready"},
		{ID: "another-gate", PublicID: 8},
		{ID: "G#99", PublicID: 9},
	}
	for reference, expected := range map[string]string{
		"stable-gate-id": "stable-gate-id",
		"G#7":            "stable-gate-id",
		"g#7":            "stable-gate-id",
		"RELEASE-READY":  "stable-gate-id",
		"G#8":            "another-gate",
	} {
		if gate := FindGateByReference(gates, reference); gate == nil || gate.ID != expected {
			t.Fatalf("reference %q resolved to %#v", reference, gate)
		}
	}
	if gate := FindGateByReference(gates, "G#99"); gate != nil {
		t.Fatalf("unknown public reference resolved to %#v", gate)
	}
}
