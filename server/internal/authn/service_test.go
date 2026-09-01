package authn

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

type sessionStoreStub struct {
	account       AccountCredential
	record        SessionRecord
	created       NewSession
	touchCount    int
	lastTouchSeen time.Time
	lastTouchIdle time.Time
}

func (s *sessionStoreStub) AccountCredentialByLogin(context.Context, string) (AccountCredential, error) {
	return s.account, nil
}
func (*sessionStoreStub) LoginRateLimited(context.Context, []byte, time.Time) (bool, error) {
	return false, nil
}
func (*sessionStoreStub) RecordLoginFailure(context.Context, []byte, time.Time) error { return nil }
func (*sessionStoreStub) RecordAuthenticationFailure(context.Context, string, time.Time) error {
	return nil
}
func (*sessionStoreStub) ClearLoginFailures(context.Context, []byte) error { return nil }
func (s *sessionStoreStub) CreateSession(_ context.Context, value NewSession) error {
	s.created = value
	s.record = SessionRecord{
		ID:                value.ID,
		AccountID:         value.AccountID,
		ActorID:           s.account.ActorID,
		LoginID:           s.account.LoginID,
		DisplayName:       s.account.DisplayName,
		AccountStatus:     s.account.Status,
		TokenHash:         append([]byte(nil), value.TokenHash...),
		CSRFTokenHash:     append([]byte(nil), value.CSRFTokenHash...),
		LastSeenAt:        value.CreatedAt,
		IdleExpiresAt:     value.IdleExpiresAt,
		AbsoluteExpiresAt: value.AbsoluteAt,
	}
	return nil
}
func (s *sessionStoreStub) SessionByTokenHash(_ context.Context, hash []byte) (SessionRecord, error) {
	if !bytes.Equal(hash, s.record.TokenHash) {
		return SessionRecord{}, errors.New("session not found")
	}
	return s.record, nil
}
func (s *sessionStoreStub) TouchSession(_ context.Context, id string, seen, idle time.Time) error {
	if id != s.record.ID {
		return errors.New("session not found")
	}
	s.touchCount++
	s.lastTouchSeen, s.lastTouchIdle = seen, idle
	s.record.LastSeenAt, s.record.IdleExpiresAt = seen, idle
	return nil
}
func (s *sessionStoreStub) RevokeSession(context.Context, string, time.Time) error { return nil }
func (*sessionStoreStub) ReplacePassword(context.Context, string, string, time.Time) error {
	return nil
}
func (*sessionStoreStub) RevokeAccountSessions(context.Context, string, time.Time) error { return nil }
func (s *sessionStoreStub) AccountCredentialByID(context.Context, string) (AccountCredential, error) {
	return s.account, nil
}
func (*sessionStoreStub) ChangePasswordAndRevokeSessions(context.Context, string, string, time.Time) error {
	return nil
}
func (*sessionStoreStub) AgentByTokenHash(context.Context, []byte, time.Time) (AgentTokenRecord, error) {
	return AgentTokenRecord{}, errors.New("agent not found")
}

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

func TestSessionIdleRefreshSlidesAndNeverPassesAbsoluteExpiry(t *testing.T) {
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	store := &sessionStoreStub{account: AccountCredential{
		AccountID: "account-1", ActorID: "actor-1", LoginID: "oidc:account-1", DisplayName: "Owner", Status: "active",
	}}
	service, err := NewServiceWithPolicy(store, SessionPolicy{IdleTTL: 90 * time.Minute, AbsoluteTTL: 2 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return base }
	result, err := service.CreateSessionForAccount(context.Background(), store.account)
	if err != nil {
		t.Fatal(err)
	}
	if !store.created.IdleExpiresAt.Equal(base.Add(90*time.Minute)) || !store.created.AbsoluteAt.Equal(base.Add(2*time.Hour)) {
		t.Fatalf("issued expiry = %s/%s", store.created.IdleExpiresAt, store.created.AbsoluteAt)
	}

	service.now = func() time.Time { return base.Add(time.Hour) }
	_, refreshed, err := service.AuthenticateSession(context.Background(), result.SessionToken)
	if err != nil {
		t.Fatal(err)
	}
	if store.touchCount != 1 || !refreshed.IdleExpiresAt.Equal(store.created.AbsoluteAt) {
		t.Fatalf("refresh count/idle = %d/%s, want one touch capped at %s", store.touchCount, refreshed.IdleExpiresAt, store.created.AbsoluteAt)
	}

	service.now = func() time.Time { return store.created.AbsoluteAt }
	if _, _, err = service.AuthenticateSession(context.Background(), result.SessionToken); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("session accepted at exact absolute boundary: %v", err)
	}
}

func TestSessionRejectsExactIdleBoundary(t *testing.T) {
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	store := &sessionStoreStub{account: AccountCredential{AccountID: "account-1", ActorID: "actor-1", Status: "active"}}
	service, err := NewServiceWithPolicy(store, SessionPolicy{IdleTTL: 30 * time.Minute, AbsoluteTTL: 2 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return base }
	result, err := service.CreateSessionForAccount(context.Background(), store.account)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return base.Add(30 * time.Minute) }
	if _, _, err = service.AuthenticateSession(context.Background(), result.SessionToken); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("session accepted at exact idle boundary: %v", err)
	}
}

func TestExistingSessionSurvivesServiceRestartWithFixedAbsoluteAndCurrentIdlePolicy(t *testing.T) {
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	store := &sessionStoreStub{account: AccountCredential{AccountID: "account-1", ActorID: "actor-1", Status: "active"}}
	issuer, err := NewServiceWithPolicy(store, SessionPolicy{IdleTTL: 30 * time.Minute, AbsoluteTTL: 2 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	issuer.now = func() time.Time { return base }
	result, err := issuer.CreateSessionForAccount(context.Background(), store.account)
	if err != nil {
		t.Fatal(err)
	}
	originalAbsolute := store.created.AbsoluteAt

	restarted, err := NewServiceWithPolicy(store, SessionPolicy{IdleTTL: 45 * time.Minute, AbsoluteTTL: 3 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	restarted.now = func() time.Time { return base.Add(10 * time.Minute) }
	_, record, err := restarted.AuthenticateSession(context.Background(), result.SessionToken)
	if err != nil {
		t.Fatal(err)
	}
	if !record.IdleExpiresAt.Equal(base.Add(55 * time.Minute)) {
		t.Fatalf("idle expiry = %s, want current-policy sliding expiry", record.IdleExpiresAt)
	}
	if !record.AbsoluteExpiresAt.Equal(originalAbsolute) {
		t.Fatalf("absolute expiry changed across restart: %s -> %s", originalAbsolute, record.AbsoluteExpiresAt)
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
