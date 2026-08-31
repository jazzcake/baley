package authn

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/authz"
)

var (
	ErrInvalidCredentials = errors.New("invalid login ID or password")
	ErrInvalidPassword    = errors.New("password must contain 15 to 64 Unicode code points")
	ErrRateLimited        = errors.New("too many login attempts")
	ErrSessionInvalid     = errors.New("session is invalid or expired")
	ErrCSRFMismatch       = errors.New("CSRF validation failed")
	ErrHashCapacity       = errors.New("password verification capacity exhausted")
)

type AccountCredential struct {
	AccountID   string
	ActorID     string
	LoginID     string
	DisplayName string
	Status      string
	PasswordPHC string
}

type SessionRecord struct {
	ID                string
	AccountID         string
	ActorID           string
	LoginID           string
	DisplayName       string
	AccountStatus     string
	TokenHash         []byte
	CSRFTokenHash     []byte
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
}

type NewSession struct {
	ID, AccountID string
	TokenHash     []byte
	CSRFTokenHash []byte
	CreatedAt     time.Time
	IdleExpiresAt time.Time
	AbsoluteAt    time.Time
}

type Store interface {
	AccountCredentialByLogin(context.Context, string) (AccountCredential, error)
	LoginRateLimited(context.Context, []byte, time.Time) (bool, error)
	RecordLoginFailure(context.Context, []byte, time.Time) error
	RecordAuthenticationFailure(context.Context, string, time.Time) error
	ClearLoginFailures(context.Context, []byte) error
	CreateSession(context.Context, NewSession) error
	SessionByTokenHash(context.Context, []byte) (SessionRecord, error)
	TouchSession(context.Context, string, time.Time, time.Time) error
	RevokeSession(context.Context, string, time.Time) error
	ReplacePassword(context.Context, string, string, time.Time) error
	RevokeAccountSessions(context.Context, string, time.Time) error
	AccountCredentialByID(context.Context, string) (AccountCredential, error)
	ChangePasswordAndRevokeSessions(context.Context, string, string, time.Time) error
	AgentByTokenHash(context.Context, []byte, time.Time) (AgentTokenRecord, error)
}

type Principal struct {
	AccountID    string
	ActorID      string
	DisplayName  string
	Subject      authz.Subject
	SessionID    string
	CredentialID string
	WorkspaceID  string
}

type AgentTokenRecord struct {
	TokenID, ActorID, WorkspaceID string
	Scopes                        []authz.Capability
}

type LoginResult struct {
	Principal
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

type Service struct {
	store       Store
	hasher      PasswordHasher
	dummyPHC    string
	now         func() time.Time
	idleTTL     time.Duration
	absoluteTTL time.Duration
}

func NewService(store Store) (*Service, error) {
	hasher := PasswordHasher{}
	dummy, err := hasher.Hash("baley-dummy-password")
	if err != nil {
		return nil, err
	}
	return &Service{store: store, hasher: hasher, dummyPHC: dummy, now: time.Now, idleTTL: 30 * time.Minute, absoluteTTL: 12 * time.Hour}, nil
}

func (s *Service) Login(ctx context.Context, loginID, password, remoteAddress string) (LoginResult, error) {
	normalized, err := NormalizeLogin(loginID)
	if err != nil || len(password) > maxPasswordBytes {
		return LoginResult{}, ErrInvalidCredentials
	}
	rateKeys := loginRateKeys(normalized, remoteAddress)
	for _, candidate := range rateKeys {
		blocked, rateErr := s.store.LoginRateLimited(ctx, candidate, s.now().UTC())
		if rateErr != nil {
			return LoginResult{}, rateErr
		}
		if blocked {
			return LoginResult{}, ErrRateLimited
		}
	}
	account, lookupErr := s.store.AccountCredentialByLogin(ctx, normalized)
	phc := s.dummyPHC
	if lookupErr == nil {
		phc = account.PasswordPHC
	}
	valid := s.hasher.Verify(phc, password)
	if lookupErr != nil || !valid || account.Status != "active" {
		for _, candidate := range rateKeys {
			if rateErr := s.store.RecordLoginFailure(ctx, candidate, s.now().UTC()); rateErr != nil {
				return LoginResult{}, rateErr
			}
		}
		loginDigest := sha256.Sum256([]byte(normalized))
		if auditErr := s.store.RecordAuthenticationFailure(ctx, base64.RawURLEncoding.EncodeToString(loginDigest[:]), s.now().UTC()); auditErr != nil {
			return LoginResult{}, auditErr
		}
		return LoginResult{}, ErrInvalidCredentials
	}
	for _, candidate := range rateKeys {
		if err = s.store.ClearLoginFailures(ctx, candidate); err != nil {
			return LoginResult{}, err
		}
	}
	if s.hasher.NeedsRehash(account.PasswordPHC) {
		replacement, hashErr := s.hasher.Hash(password)
		if hashErr != nil {
			return LoginResult{}, hashErr
		}
		if err = s.store.ReplacePassword(ctx, account.AccountID, replacement, s.now().UTC()); err != nil {
			return LoginResult{}, err
		}
	}
	return s.CreateSessionForAccount(ctx, account)
}

// CreateSessionForAccount issues the same bounded, CSRF-protected local
// session for a password or OIDC-authenticated account. Authentication method
// selection therefore cannot broaden human-only authorization capabilities.
func (s *Service) CreateSessionForAccount(ctx context.Context, account AccountCredential) (LoginResult, error) {
	if account.Status != "active" || account.AccountID == "" || account.ActorID == "" {
		return LoginResult{}, ErrInvalidCredentials
	}
	sessionToken, sessionHash, err := randomSecret()
	if err != nil {
		return LoginResult{}, err
	}
	csrfToken, csrfHash, err := randomSecret()
	if err != nil {
		return LoginResult{}, err
	}
	now := s.now().UTC()
	sessionID, err := randomID()
	if err != nil {
		return LoginResult{}, err
	}
	session := NewSession{ID: sessionID, AccountID: account.AccountID, TokenHash: sessionHash, CSRFTokenHash: csrfHash, CreatedAt: now, IdleExpiresAt: now.Add(s.idleTTL), AbsoluteAt: now.Add(s.absoluteTTL)}
	if err = s.store.CreateSession(ctx, session); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		Principal: Principal{AccountID: account.AccountID, ActorID: account.ActorID, DisplayName: account.DisplayName, SessionID: sessionID,
			Subject: authz.Subject{ActorID: account.ActorID, Kind: authz.ActorHuman, Credential: authz.HumanSession, Scopes: append([]authz.Capability(nil), authz.Capabilities...)}},
		SessionToken: sessionToken, CSRFToken: csrfToken, ExpiresAt: session.AbsoluteAt,
	}, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, token string) (Principal, SessionRecord, error) {
	hash := sha256.Sum256([]byte(token))
	record, err := s.store.SessionByTokenHash(ctx, hash[:])
	now := s.now().UTC()
	if err != nil || record.AccountStatus != "active" || record.RevokedAt != nil || !now.Before(record.IdleExpiresAt) || !now.Before(record.AbsoluteExpiresAt) {
		return Principal{}, SessionRecord{}, ErrSessionInvalid
	}
	if now.Sub(record.LastSeenAt) >= time.Minute {
		idle := now.Add(s.idleTTL)
		if idle.After(record.AbsoluteExpiresAt) {
			idle = record.AbsoluteExpiresAt
		}
		if err = s.store.TouchSession(ctx, record.ID, now, idle); err != nil {
			return Principal{}, SessionRecord{}, err
		}
		record.LastSeenAt, record.IdleExpiresAt = now, idle
	}
	return Principal{AccountID: record.AccountID, ActorID: record.ActorID, DisplayName: record.DisplayName, SessionID: record.ID,
		Subject: authz.Subject{ActorID: record.ActorID, Kind: authz.ActorHuman, Credential: authz.HumanSession, Scopes: append([]authz.Capability(nil), authz.Capabilities...)}}, record, nil
}

func (s *Service) AuthenticateBearer(ctx context.Context, token string) (Principal, error) {
	hash := sha256.Sum256([]byte(token))
	record, err := s.store.AgentByTokenHash(ctx, hash[:], s.now().UTC())
	if err != nil {
		return Principal{}, ErrSessionInvalid
	}
	return Principal{ActorID: record.ActorID, CredentialID: record.TokenID, WorkspaceID: record.WorkspaceID,
		Subject: authz.Subject{ActorID: record.ActorID, Kind: authz.ActorAgent, Credential: authz.AgentToken, Scopes: record.Scopes}}, nil
}

func (s *Service) VerifyCSRF(record SessionRecord, token string) error {
	hash := sha256.Sum256([]byte(token))
	if len(record.CSRFTokenHash) != len(hash) || !equalBytes(record.CSRFTokenHash, hash[:]) {
		return ErrCSRFMismatch
	}
	return nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.store.RevokeSession(ctx, sessionID, s.now().UTC())
}

func (s *Service) HashPassword(password string) (string, error) {
	return s.hasher.Hash(password)
}

func (s *Service) ChangePassword(ctx context.Context, accountID, currentPassword, nextPassword string) error {
	account, err := s.store.AccountCredentialByID(ctx, accountID)
	if err != nil || !s.hasher.Verify(account.PasswordPHC, currentPassword) {
		return ErrInvalidCredentials
	}
	replacement, err := s.hasher.Hash(nextPassword)
	if err != nil {
		return err
	}
	return s.store.ChangePasswordAndRevokeSessions(ctx, accountID, replacement, s.now().UTC())
}

func loginRateKeys(login, remoteAddress string) [][]byte {
	host := strings.TrimSpace(remoteAddress)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	loginHash := sha256.Sum256([]byte("baley-login-account-v1\x00" + login))
	accountPeerHash := sha256.Sum256([]byte("baley-login-account-peer-v1\x00" + login + "\x00" + host))
	return [][]byte{loginHash[:], accountPeerHash[:]}
}

func randomSecret() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func randomID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}
