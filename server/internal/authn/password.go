package authn

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

const (
	minPasswordRunes = 15
	maxPasswordRunes = 64
	maxPasswordBytes = 512
	argonMemoryKiB   = 19 * 1024
	argonIterations  = 2
	argonParallelism = 1
	argonSaltBytes   = 16
	argonOutputBytes = 32
)

type PasswordHasher struct{}

var argonSlots = make(chan struct{}, 2)

func acquireArgon() bool {
	select {
	case argonSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseArgon() { <-argonSlots }

func NormalizeLogin(value string) (string, error) {
	normalized := strings.ToLower(norm.NFKC.String(strings.TrimSpace(value)))
	if normalized == "" || len(normalized) > 128 || !utf8.ValidString(normalized) {
		return "", ErrInvalidCredentials
	}
	return normalized, nil
}

func ValidatePassword(value string) error {
	if len(value) > maxPasswordBytes || !utf8.ValidString(value) {
		return ErrInvalidPassword
	}
	count := utf8.RuneCountInString(value)
	if count < minPasswordRunes || count > maxPasswordRunes {
		return ErrInvalidPassword
	}
	return nil
}

func (PasswordHasher) Hash(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	if !acquireArgon() {
		return "", ErrHashCapacity
	}
	defer releaseArgon()
	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonOutputBytes)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemoryKiB, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func (PasswordHasher) Verify(phc, password string) bool {
	if len(password) > maxPasswordBytes || !utf8.ValidString(password) {
		return false
	}
	memory, iterations, parallelism, salt, expected, ok := parsePHC(phc)
	if !ok {
		return false
	}
	if !acquireArgon() {
		return false
	}
	defer releaseArgon()
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (PasswordHasher) NeedsRehash(phc string) bool {
	memory, iterations, parallelism, salt, expected, ok := parsePHC(phc)
	return !ok || memory != argonMemoryKiB || iterations != argonIterations ||
		parallelism != argonParallelism || len(salt) != argonSaltBytes || len(expected) != argonOutputBytes
}

func parsePHC(value string) (uint32, uint32, uint8, []byte, []byte, bool) {
	parts := strings.Split(value, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return 0, 0, 0, nil, nil, false
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return 0, 0, 0, nil, nil, false
	}
	parse := func(raw, prefix string, max uint64) (uint64, bool) {
		if !strings.HasPrefix(raw, prefix) {
			return 0, false
		}
		value, err := strconv.ParseUint(strings.TrimPrefix(raw, prefix), 10, 32)
		return value, err == nil && value > 0 && value <= max
	}
	memory, memoryOK := parse(params[0], "m=", 64*1024)
	iterations, iterationsOK := parse(params[1], "t=", 4)
	parallelism, parallelismOK := parse(params[2], "p=", 4)
	if !memoryOK || !iterationsOK || !parallelismOK || memory < 8*1024 {
		return 0, 0, 0, nil, nil, false
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[4])
	expected, hashErr := base64.RawStdEncoding.DecodeString(parts[5])
	if saltErr != nil || hashErr != nil || len(salt) < 16 || len(salt) > 32 || len(expected) < 16 || len(expected) > 64 {
		return 0, 0, 0, nil, nil, false
	}
	return uint32(memory), uint32(iterations), uint8(parallelism), salt, expected, true
}
