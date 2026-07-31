package authn

import (
	"strings"
	"testing"
)

func TestPasswordHasherRoundTripAndBoundedPHC(t *testing.T) {
	hasher := PasswordHasher{}
	password := "correct horse battery staple"
	phc, err := hasher.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	if !hasher.Verify(phc, password) || hasher.Verify(phc, password+"x") {
		t.Fatal("password verification mismatch")
	}
	oversizedParams := strings.Replace(phc, "m=19456", "m=999999999", 1)
	if hasher.Verify(oversizedParams, password) {
		t.Fatal("unbounded Argon2 parameters accepted")
	}
}

func TestPasswordAndLoginBounds(t *testing.T) {
	if ValidatePassword("short") == nil {
		t.Fatal("short password accepted")
	}
	if ValidatePassword(strings.Repeat("가", 65)) == nil {
		t.Fatal("long password accepted")
	}
	if got, err := NormalizeLogin("  ＯＷＮＥＲ  "); err != nil || got != "owner" {
		t.Fatalf("normalized login=%q err=%v", got, err)
	}
}
