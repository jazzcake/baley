package authn

import (
	"bytes"
	"testing"
)

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
