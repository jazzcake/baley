package authn

import (
	"bytes"
	"testing"
	"time"
)

func TestDefaultSessionPolicySupportsLongLivedLogin(t *testing.T) {
	policy := DefaultSessionPolicy()
	if policy.IdleTTL != 30*24*time.Hour || policy.AbsoluteTTL != 90*24*time.Hour {
		t.Fatalf("unexpected default session policy: %#v", policy)
	}
	service, err := NewServiceWithPolicy(nil, policy)
	if err != nil {
		t.Fatal(err)
	}
	if service.idleTTL != policy.IdleTTL || service.absoluteTTL != policy.AbsoluteTTL {
		t.Fatalf("service policy = %s/%s, want %s/%s", service.idleTTL, service.absoluteTTL, policy.IdleTTL, policy.AbsoluteTTL)
	}
}

func TestSessionPolicyRejectsUnsafeBounds(t *testing.T) {
	tests := []SessionPolicy{
		{IdleTTL: 0, AbsoluteTTL: time.Hour},
		{IdleTTL: time.Hour, AbsoluteTTL: 0},
		{IdleTTL: 2 * time.Hour, AbsoluteTTL: time.Hour},
		{IdleTTL: 30 * 24 * time.Hour, AbsoluteTTL: 366 * 24 * time.Hour},
	}
	for _, policy := range tests {
		if err := ValidateSessionPolicy(policy); err == nil {
			t.Fatalf("unsafe policy accepted: %#v", policy)
		}
	}
}

func TestLoginRateKeysDoNotCreateSharedProxyLockout(t *testing.T) {
	firstAccount := loginRateKeys("first-account", "127.0.0.1:41000")
	secondAccount := loginRateKeys("second-account", "127.0.0.1:41001")
	if len(firstAccount) != 2 || len(secondAccount) != 2 {
		t.Fatalf("unexpected rate key count: %d / %d", len(firstAccount), len(secondAccount))
	}
	for _, firstKey := range firstAccount {
		for _, secondKey := range secondAccount {
			if bytes.Equal(firstKey, secondKey) {
				t.Fatal("different accounts behind one proxy share a login rate-limit key")
			}
		}
	}

	sameAccountOtherPeer := loginRateKeys("first-account", "127.0.0.2:42000")
	if !bytes.Equal(firstAccount[0], sameAccountOtherPeer[0]) {
		t.Fatal("per-account limit changed with the transport peer")
	}
	if bytes.Equal(firstAccount[1], sameAccountOtherPeer[1]) {
		t.Fatal("combined account/peer limit ignored the transport peer")
	}
}
