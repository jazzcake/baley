package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/authn"
	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

type API struct {
	Service          *application.Service
	Repo             *postgres.Repository
	AllowedOrigins   []string
	ApprovalOrigin   string
	Auth             *authn.Service
	OIDC             *authn.OIDCService
	OIDCPostLoginURL string
	AuthMode         string
	CookieSecure     bool
	Build            BuildInfo
	ReadyCheck       func(context.Context) (int64, error)
}

type BuildInfo struct {
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	BuiltAt       string `json:"builtAt"`
	SchemaVersion int64  `json:"schemaVersion"`
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", a.readiness)
	mux.HandleFunc("GET /versionz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, a.Build) })
	mux.HandleFunc("POST /v1/auth/login", a.login)
	mux.HandleFunc("GET /v1/auth/oidc/providers", a.oidcProviders)
	mux.HandleFunc("GET /v1/auth/oidc/{providerId}/start", a.oidcStart)
	mux.HandleFunc("POST /v1/auth/oidc/{providerId}/link", a.oidcLink)
	mux.HandleFunc("GET /v1/auth/oidc/{providerId}/callback", a.oidcCallback)
	mux.HandleFunc("GET /v1/auth/principal", a.authPrincipal)
	mux.HandleFunc("GET /v1/auth/session", a.authSession)
	mux.HandleFunc("POST /v1/auth/logout", a.logout)
	mux.HandleFunc("POST /v1/auth/password", a.changePassword)
	mux.HandleFunc("POST /v1/mcp/connections", a.createMCPConnection)
	mux.HandleFunc("GET /v1/mcp/connections/{connectionId}", a.pollMCPConnection)
	mux.HandleFunc("POST /v1/mcp/gateway-sessions", a.resumeMCPGateway)
	mux.HandleFunc("GET /v1/workspaces", a.workspaces)
	mux.HandleFunc("POST /v1/workspaces", a.createWorkspace)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/members", a.members)
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/members", a.createMember)
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/memberships", a.addExistingMember)
	mux.HandleFunc("PATCH /v1/workspaces/{workspaceId}/members/{actorId}", a.updateMember)
	mux.HandleFunc("DELETE /v1/workspaces/{workspaceId}/members/{actorId}", a.removeMember)
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/members/{actorId}/disable-account", a.disableMemberAccount)
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/members/{actorId}/reset-password", a.resetMemberPassword)
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/owner-transfer", a.ownerTransfer)
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/agent-tokens", a.issueAgentToken)
	mux.HandleFunc("DELETE /v1/workspaces/{workspaceId}/agent-tokens/{tokenId}", a.revokeAgentToken)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/mcp-connections/{connectionId}", a.getMCPConnection)
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/mcp-connections/{connectionId}/approve", a.approveMCPConnection)
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/mcp-connections/{connectionId}/reject", a.rejectMCPConnection)
	mux.HandleFunc("DELETE /v1/workspaces/{workspaceId}/mcp-gateways/{gatewayId}", a.revokeMCPGateway)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}", a.workspace)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/context", a.workspaceContext)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/graph", a.graph)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/phases/{phaseId}/tasks", a.phaseTasks)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/tasks/{publicId}", a.task)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/tasks/{publicId}/acceptance", a.taskAcceptance)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/lanes/{laneId}/brief", a.laneBrief)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/backlog", a.backlogList)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/backlog/{publicId}", a.backlog)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/gates/{gateId}/status", a.gate)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/decisions", a.decisions)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/events", a.events)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/mutation-attempts", a.mutationAttempts)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/runs", a.runs)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/records", a.records)
	mux.HandleFunc("POST /v1/commands/preview", a.preview)
	mux.HandleFunc("POST /v1/commands/execute", a.execute)
	return a.observability(a.cors(a.authentication(mux)))
}

func (a *API) oidcProviders(w http.ResponseWriter, _ *http.Request) {
	if a.AuthMode != "enforced" || a.OIDC == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": a.OIDC.Providers()})
}

func (a *API) oidcStart(w http.ResponseWriter, r *http.Request) {
	if a.AuthMode != "enforced" || a.OIDC == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "OIDC is not configured"}})
		return
	}
	binding, _, err := oidcCookieSecret()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "oidc_start_failed", "message": "could not start OIDC login"}})
		return
	}
	url, err := a.OIDC.Start(r.Context(), r.PathValue("providerId"), binding, "login", "")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "oidc_start_failed", "message": "OIDC provider is unavailable"}})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: a.oidcBindingCookieName(), Value: binding, Path: "/", HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	http.Redirect(w, r, url, http.StatusFound)
}

func (a *API) oidcLink(w http.ResponseWriter, r *http.Request) {
	state, ok := authState(r)
	if !ok || state.Principal.AccountID == "" || a.OIDC == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": "human session required"}})
		return
	}
	binding, _, err := oidcCookieSecret()
	if err != nil {
		writeError(w, err)
		return
	}
	url, err := a.OIDC.Start(r.Context(), r.PathValue("providerId"), binding, "link", state.Principal.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "oidc_link_failed", "message": "OIDC provider is unavailable"}})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: a.oidcBindingCookieName(), Value: binding, Path: "/", HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	writeJSON(w, http.StatusOK, map[string]string{"authorizationUrl": url})
}

func (a *API) oidcCallback(w http.ResponseWriter, r *http.Request) {
	postLogin := a.oidcPostLoginURL(false)
	if a.OIDC == nil || r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, a.oidcPostLoginURL(true), http.StatusSeeOther)
		return
	}
	bindingCookie, err := r.Cookie(a.oidcBindingCookieName())
	if err != nil {
		http.Redirect(w, r, a.oidcPostLoginURL(true), http.StatusSeeOther)
		return
	}
	result, _, err := a.OIDC.Complete(r.Context(), r.PathValue("providerId"), r.URL.Query().Get("state"), bindingCookie.Value, r.URL.Query().Get("code"))
	http.SetCookie(w, &http.Cookie{Name: a.oidcBindingCookieName(), Path: "/", HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	if err != nil {
		log.Printf("OIDC callback rejected for provider %q: %v", r.PathValue("providerId"), err)
		http.Redirect(w, r, a.oidcPostLoginURL(true), http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: a.sessionCookieName(), Value: result.SessionToken, Path: "/", HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: result.ExpiresAt})
	http.SetCookie(w, &http.Cookie{Name: a.csrfCookieName(), Value: result.CSRFToken, Path: "/", HttpOnly: false, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: result.ExpiresAt})
	http.Redirect(w, r, postLogin, http.StatusSeeOther)
}

func (a *API) oidcPostLoginURL(failed bool) string {
	base := strings.TrimRight(a.OIDCPostLoginURL, "/")
	if base == "" && len(a.AllowedOrigins) > 0 {
		base = strings.TrimRight(a.AllowedOrigins[0], "/") + "/workspaces"
	}
	if base == "" {
		base = "/workspaces"
	}
	if failed {
		return base + "?oidc=failed"
	}
	return base
}

func (a *API) oidcBindingCookieName() string {
	if a.CookieSecure {
		return "__Host-baley_oidc_binding"
	}
	return "baley_oidc_binding"
}
func oidcCookieSecret() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hash[:], nil
}

func (a *API) readiness(w http.ResponseWriter, r *http.Request) {
	if a.ReadyCheck == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "error": "readiness check unavailable"})
		return
	}
	schemaVersion, err := a.ReadyCheck(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "schemaVersion": schemaVersion, "version": a.buildVersion()})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if a.AuthMode != "enforced" || a.Auth == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "authentication is not enabled"}})
		return
	}
	if !a.isOriginAllowed(r.Header.Get("Origin")) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "origin_denied", "message": "request origin is not allowed"}})
		return
	}
	var input struct {
		LoginID  string `json:"loginId"`
		Password string `json:"password"`
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2048))
	if err != nil || decodeStrict(raw, &input) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid login request"}})
		return
	}
	result, err := a.Auth.Login(r.Context(), input.LoginID, input.Password, r.RemoteAddr)
	if err != nil {
		status, code := http.StatusUnauthorized, "invalid_credentials"
		if errors.Is(err, authn.ErrRateLimited) {
			status, code = http.StatusTooManyRequests, "rate_limited"
		}
		writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": "invalid login ID or password"}})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: a.sessionCookieName(), Value: result.SessionToken, Path: "/", HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: result.ExpiresAt})
	http.SetCookie(w, &http.Cookie{Name: a.csrfCookieName(), Value: result.CSRFToken, Path: "/", HttpOnly: false, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: result.ExpiresAt})
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"account":       map[string]any{"id": result.AccountID, "actorId": result.ActorID, "loginId": input.LoginID, "displayName": result.DisplayName},
		"csrfToken":     result.CSRFToken, "expiresAt": result.ExpiresAt,
	})
}

func (a *API) authSession(w http.ResponseWriter, r *http.Request) {
	state, ok := authState(r)
	if !ok || state.Principal.AccountID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": "authentication required"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"account":       map[string]any{"id": state.Principal.AccountID, "actorId": state.Principal.ActorID, "loginId": state.Session.LoginID, "displayName": state.Principal.DisplayName},
		"csrfToken":     state.CSRFToken, "expiresAt": state.Session.AbsoluteExpiresAt,
	})
}

// authPrincipal is a deliberately minimal credential validation endpoint for
// local Streamable HTTP MCP frontends. It returns no token material and is
// protected by the ordinary authentication middleware.
func (a *API) authPrincipal(w http.ResponseWriter, r *http.Request) {
	state, ok := authState(r)
	if !ok || state.Principal.ActorID == "" || state.Principal.Subject.Kind != authz.ActorAgent {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": "authentication required"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "actorId": state.Principal.ActorID, "workspaceId": state.Principal.WorkspaceID, "subjectKind": state.Principal.Subject.Kind})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	state, ok := authState(r)
	if !ok || state.Principal.SessionID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": "human session required"}})
		return
	}
	if err := a.Auth.Logout(r.Context(), state.Principal.SessionID); err != nil {
		writeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: a.sessionCookieName(), Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: a.csrfCookieName(), Path: "/", MaxAge: -1, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) changePassword(w http.ResponseWriter, r *http.Request) {
	state, ok := authState(r)
	if !ok || state.Principal.AccountID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": "human session required"}})
		return
	}
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := a.Auth.ChangePassword(r.Context(), state.Principal.AccountID, input.CurrentPassword, input.NewPassword); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "password_change_failed", "message": err.Error()}})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: a.sessionCookieName(), Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: a.csrfCookieName(), Path: "/", MaxAge: -1, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) workspaces(w http.ResponseWriter, r *http.Request) {
	state, ok := authState(r)
	if !ok || state.Principal.AccountID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": "human session required"}})
		return
	}
	values, err := a.Repo.ListAccountWorkspaces(r.Context(), state.Principal.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(values))
	for _, value := range values {
		relationship := "participant"
		if value.Role == string(authz.RoleOwner) {
			relationship = "owner"
		}
		items = append(items, map[string]any{"id": value.ID, "name": value.Name, "state": value.State, "revision": value.Revision, "role": value.Role, "relationship": relationship, "capabilities": value.Capabilities})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) createWorkspace(w http.ResponseWriter, r *http.Request) {
	state, ok := authState(r)
	if !ok || state.Principal.AccountID == "" || state.Principal.Subject.Kind != authz.ActorHuman {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": "human session required"}})
		return
	}
	var input struct {
		WorkspaceID string `json:"workspaceId"`
		Name        string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !isLowerUUID(input.WorkspaceID) || strings.TrimSpace(input.Name) == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_workspace", "message": "lowercase UUID and non-empty name are required"}})
		return
	}
	value, err := a.Repo.CreateOwnedWorkspace(r.Context(), input.WorkspaceID, input.Name, state.Principal.ActorID)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "workspace_create_failed", "message": err.Error()}})
		return
	}
	status := http.StatusCreated
	if value.Idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"id": value.ID, "name": value.Name, "state": value.State, "revision": value.Revision,
		"role": value.Role, "relationship": "owner", "capabilities": value.Capabilities,
		"idempotent": value.Idempotent,
	})
}

func isLowerUUID(value string) bool {
	if len(value) != 36 || value != strings.ToLower(value) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func (a *API) members(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireOwner(r); !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	values, err := a.Repo.ListMembers(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(values))
	for _, value := range values {
		relationship := "participant"
		if value.Role == string(authz.RoleOwner) {
			relationship = "owner"
		}
		items = append(items, map[string]any{"actorId": value.ActorID, "accountId": value.AccountID, "displayName": value.DisplayName, "role": value.Role, "relationship": relationship, "active": value.Active})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) requireOwner(r *http.Request) (authContext, bool) {
	state, ok := authState(r)
	if !ok || state.Principal.AccountID == "" {
		return authContext{}, false
	}
	membership, err := a.Repo.Membership(r.Context(), r.PathValue("workspaceId"), state.Principal.ActorID)
	return state, err == nil && membership != nil && membership.Active && membership.Role == authz.RoleOwner
}

func (a *API) createMember(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var input struct {
		LoginID         string     `json:"loginId"`
		DisplayName     string     `json:"displayName"`
		InitialPassword string     `json:"initialPassword"`
		Role            authz.Role `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	normalized, err := authn.NormalizeLogin(input.LoginID)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_account", "message": err.Error()}})
		return
	}
	phc, err := a.Auth.HashPassword(input.InitialPassword)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_password", "message": err.Error()}})
		return
	}
	created, err := a.Repo.CreateMember(r.Context(), r.PathValue("workspaceId"), state.Principal.ActorID, input.LoginID, normalized, input.DisplayName, phc, input.Role)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "member_create_failed", "message": err.Error()}})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"accountId": created.AccountID, "actorId": created.ActorID, "displayName": input.DisplayName, "role": input.Role, "relationship": map[bool]string{true: "owner", false: "participant"}[input.Role == authz.RoleOwner], "active": true})
}

func (a *API) addExistingMember(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var input struct {
		LoginID string     `json:"loginId"`
		Role    authz.Role `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	normalized, err := authn.NormalizeLogin(input.LoginID)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_account", "message": err.Error()}})
		return
	}
	member, err := a.Repo.AddExistingMember(r.Context(), r.PathValue("workspaceId"), state.Principal.ActorID, normalized, input.Role)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "membership_create_failed", "message": err.Error()}})
		return
	}
	relationship := "participant"
	if member.Role == string(authz.RoleOwner) {
		relationship = "owner"
	}
	writeJSON(w, http.StatusCreated, map[string]any{"accountId": member.AccountID, "actorId": member.ActorID, "displayName": member.DisplayName, "role": member.Role, "relationship": relationship, "active": member.Active})
}

func (a *API) disableMemberAccount(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if err := a.Repo.DisableMemberAccount(r.Context(), r.PathValue("workspaceId"), r.PathValue("actorId"), state.Principal.ActorID); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "account_disable_failed", "message": err.Error()}})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) resetMemberPassword(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var input struct {
		NewPassword string `json:"newPassword"`
	}
	if !decode(w, r, &input) {
		return
	}
	passwordPHC, err := a.Auth.HashPassword(input.NewPassword)
	input.NewPassword = ""
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_password", "message": err.Error()}})
		return
	}
	if err = a.Repo.AdminResetMemberPassword(r.Context(), r.PathValue("workspaceId"), r.PathValue("actorId"), state.Principal.ActorID, passwordPHC); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "password_reset_failed", "message": err.Error()}})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) updateMember(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var input struct {
		Role   *authz.Role `json:"role,omitempty"`
		Active *bool       `json:"active,omitempty"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := a.Repo.UpdateMember(r.Context(), r.PathValue("workspaceId"), r.PathValue("actorId"), state.Principal.ActorID, input.Role, input.Active); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "member_update_failed", "message": err.Error()}})
		return
	}
	values, err := a.Repo.ListMembers(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	for _, value := range values {
		if value.ActorID == r.PathValue("actorId") {
			relationship := "participant"
			if value.Role == string(authz.RoleOwner) {
				relationship = "owner"
			}
			writeJSON(w, http.StatusOK, map[string]any{"actorId": value.ActorID, "accountId": value.AccountID, "displayName": value.DisplayName, "role": value.Role, "relationship": relationship, "active": value.Active})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "member not found"}})
}

func (a *API) removeMember(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	active := false
	if err := a.Repo.UpdateMember(r.Context(), r.PathValue("workspaceId"), r.PathValue("actorId"), state.Principal.ActorID, nil, &active); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "member_remove_failed", "message": err.Error()}})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) ownerTransfer(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var input struct {
		TargetActorID     string     `json:"targetActorId"`
		PreviousOwnerRole authz.Role `json:"previousOwnerRole"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := a.Repo.TransferOwner(r.Context(), r.PathValue("workspaceId"), state.Principal.ActorID, input.TargetActorID, input.PreviousOwnerRole); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "owner_transfer_failed", "message": err.Error()}})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) issueAgentToken(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var input struct {
		ActorID   string             `json:"actorId"`
		Name      string             `json:"name"`
		Scopes    []authz.Capability `json:"scopes,omitempty"`
		ExpiresAt *time.Time         `json:"expiresAt,omitempty"`
	}
	if !decode(w, r, &input) {
		return
	}
	result, err := a.Repo.IssueAgentToken(r.Context(), r.PathValue("workspaceId"), input.ActorID, input.Name, state.Principal.ActorID, input.Scopes, input.ExpiresAt)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "agent_token_issue_failed", "message": err.Error()}})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": result.ID, "token": result.Token, "prefix": result.Prefix})
}

func (a *API) revokeAgentToken(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	if err := a.Repo.RevokeAgentToken(r.Context(), r.PathValue("workspaceId"), r.PathValue("tokenId"), state.Principal.ActorID); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "agent_token_revoke_failed", "message": err.Error()}})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type authContext struct {
	Principal authn.Principal
	Session   authn.SessionRecord
	Cookie    bool
	CSRFToken string
}

type authContextKey struct{}

func (a *API) authentication(next http.Handler) http.Handler {
	if a.AuthMode != "enforced" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/versionz" || r.URL.Path == "/v1/auth/login" || r.URL.Path == "/v1/auth/oidc/providers" || strings.HasSuffix(r.URL.Path, "/start") && strings.HasPrefix(r.URL.Path, "/v1/auth/oidc/") || strings.HasSuffix(r.URL.Path, "/callback") && strings.HasPrefix(r.URL.Path, "/v1/auth/oidc/") || strings.HasPrefix(r.URL.Path, "/v1/mcp/connections") {
			next.ServeHTTP(w, r)
			return
		}
		if a.Auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]string{"code": "auth_unavailable", "message": "authentication is unavailable"}})
			return
		}
		var state authContext
		var err error
		if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
			state.Principal, err = a.Auth.AuthenticateBearer(r.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		} else if cookie, cookieErr := r.Cookie(a.sessionCookieName()); cookieErr == nil {
			state.Principal, state.Session, err = a.Auth.AuthenticateSession(r.Context(), cookie.Value)
			state.Cookie = true
			if csrfCookie, csrfErr := r.Cookie(a.csrfCookieName()); csrfErr == nil {
				state.CSRFToken = csrfCookie.Value
			}
		} else {
			err = authn.ErrSessionInvalid
		}
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": "authentication required"}})
			return
		}
		if state.Principal.WorkspaceID != "" {
			workspaceID := pathWorkspaceID(r.URL.Path)
			if workspaceID != "" && workspaceID != state.Principal.WorkspaceID {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "workspace not found"}})
				return
			}
		}
		if state.Cookie && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if !a.isOriginAllowed(r.Header.Get("Origin")) || state.CSRFToken == "" ||
				r.Header.Get("X-Baley-CSRF") != state.CSRFToken || a.Auth.VerifyCSRF(state.Session, state.CSRFToken) != nil {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "csrf_mismatch", "message": "request origin or CSRF token is invalid"}})
				return
			}
		}
		if workspaceID := pathWorkspaceID(r.URL.Path); workspaceID != "" {
			membership, membershipErr := a.Repo.Membership(r.Context(), workspaceID, state.Principal.ActorID)
			decision := authz.Authorize(authz.AuthorizationInput{
				Subject: state.Principal.Subject, Membership: membership, WorkspaceID: workspaceID,
				EntityWorkspaceID: workspaceID, Capability: authz.WorkspaceRead,
			})
			if membershipErr != nil || !decision.Allowed {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "workspace not found"}})
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authContextKey{}, state)))
	})
}

func pathWorkspaceID(path string) string {
	const prefix = "/v1/workspaces/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if index := strings.IndexByte(rest, '/'); index >= 0 {
		rest = rest[:index]
	}
	return strings.TrimSpace(rest)
}

func authState(r *http.Request) (authContext, bool) {
	value, ok := r.Context().Value(authContextKey{}).(authContext)
	return value, ok
}

func (a *API) sessionCookieName() string {
	if a.CookieSecure {
		return "__Host-baley_session"
	}
	return "baley_session"
}

func (a *API) csrfCookieName() string {
	if a.CookieSecure {
		return "__Host-baley_csrf"
	}
	return "baley_csrf"
}
func (a *API) laneBrief(w http.ResponseWriter, r *http.Request) {
	brief, err := a.Service.LaneBrief(r.Context(), r.PathValue("workspaceId"), r.PathValue("laneId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, brief)
}

func (a *API) workspace(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, s.Workspace)
}
func (a *API) workspaceContext(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, application.WorkspaceContext(s))
}
func (a *API) graph(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	activeBacklog := []application.BacklogItemProjection{}
	for _, item := range s.BacklogItems {
		if item.Status == "active" {
			activeBacklog = append(activeBacklog, item)
		}
	}
	writeJSON(w, 200, map[string]any{"workspace": s.Workspace, "phases": s.Phases, "lanes": s.Lanes, "tasks": s.Tasks, "backlogItems": activeBacklog, "dependencies": s.Dependencies, "gates": s.Gates, "runs": s.Runs, "repositories": s.Repositories, "records": s.Records, "commits": s.Commits, "gitObservations": s.GitObservations, "acceptancePolicy": s.AcceptancePolicy, "evidenceProfiles": s.EvidenceProfiles, "acceptanceAssignments": s.AcceptanceAssignments, "acceptanceEvidence": s.AcceptanceEvidence, "decisions": projectDecisions(s)})
}

func (a *API) phaseTasks(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	phaseID := r.PathValue("phaseId")
	found, completed := false, false
	for _, phase := range s.Phases {
		if phase.ID == phaseID {
			found, completed = true, phase.State == "completed"
			break
		}
	}
	if !found || completed {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "active phase not found"}})
		return
	}
	cursor := 0
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		cursor, err = strconv.Atoi(raw)
	}
	if err != nil || cursor < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_cursor", "message": "cursor must be a non-negative Task public ID"}})
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if r.URL.Query().Get("limit") != "" && (err != nil || limit <= 0 || limit > 100) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_limit", "message": "limit must be between 1 and 100"}})
		return
	}
	tasks, nextCursor, hasMore := application.PhaseTasksPage(s, phaseID, cursor, limit)
	writeJSON(w, http.StatusOK, map[string]any{"phaseId": phaseID, "items": tasks, "nextCursor": nextCursor, "hasMore": hasMore})
}

func (a *API) taskAcceptance(w http.ResponseWriter, r *http.Request) {
	publicID, err := strconv.Atoi(r.PathValue("publicId"))
	if err != nil || publicID <= 0 {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "publicId must be positive"}})
		return
	}
	snapshot, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	var task *application.TaskProjection
	for index := range snapshot.Tasks {
		if snapshot.Tasks[index].PublicID == publicID {
			task = &snapshot.Tasks[index]
			break
		}
	}
	if task == nil {
		writeJSON(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "task not found"}})
		return
	}
	assignments, evidence := []any{}, []any{}
	for _, value := range snapshot.AcceptanceAssignments {
		if value.TaskID == task.ID {
			assignments = append(assignments, value)
		}
	}
	for _, value := range snapshot.AcceptanceEvidence {
		if value.TaskID == task.ID {
			evidence = append(evidence, value)
		}
	}
	var profile any
	for _, value := range snapshot.EvidenceProfiles {
		if value.ID == task.EvidenceProfileID {
			profile = value
			break
		}
	}
	writeJSON(w, 200, map[string]any{"task": task, "policy": snapshot.AcceptancePolicy, "profile": profile, "assignments": assignments, "evidence": evidence})
}
func (a *API) backlog(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("publicId"))
	if err != nil || id <= 0 {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_backlog_filter", "message": "publicId must be a positive integer"}})
		return
	}
	v, err := a.Repo.Backlog(r.Context(), r.PathValue("workspaceId"), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (a *API) backlogList(w http.ResponseWriter, r *http.Request) {
	limit := 50
	after := 0
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
	}
	if err == nil {
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			after, err = strconv.Atoi(raw)
		}
	}
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_backlog_filter", "message": "invalid cursor or limit"}})
		return
	}
	items, err := a.Repo.BacklogList(r.Context(), r.PathValue("workspaceId"), r.URL.Query().Get("laneId"), r.URL.Query().Get("status"), after, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	next := ""
	if len(items) == limit {
		next = strconv.Itoa(items[len(items)-1].PublicID)
	}
	writeJSON(w, 200, map[string]any{"items": items, "nextCursor": next})
}
func (a *API) task(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("publicId"))
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "publicId must be an integer"}})
		return
	}
	v, err := a.Repo.Task(r.Context(), r.PathValue("workspaceId"), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (a *API) gate(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	if gate := application.FindGateByReference(s.Gates, r.PathValue("gateId")); gate != nil {
		writeJSON(w, 200, gate)
		return
	}
	writeJSON(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "gate not found"}})
}
func (a *API) decisions(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, projectDecisions(s))
}
func (a *API) events(w http.ResponseWriter, r *http.Request) {
	v, err := a.Repo.Events(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (a *API) mutationAttempts(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 200 {
			writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "limit must be between 1 and 200"}})
			return
		}
		limit = parsed
	}
	var after time.Time
	if raw := r.URL.Query().Get("after"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "after must be RFC3339"}})
			return
		}
		after = parsed
	}
	afterID := r.URL.Query().Get("afterId")
	if !after.IsZero() && afterID == "" {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "afterId is required with after"}})
		return
	}
	items, err := a.Repo.MutationAttempts(r.Context(), r.PathValue("workspaceId"), r.URL.Query().Get("outcome"), r.URL.Query().Get("commandName"), after, afterID, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	next := ""
	nextID := ""
	if len(items) == limit {
		next = items[len(items)-1].OccurredAt.Format(time.RFC3339Nano)
		nextID = items[len(items)-1].ID
	}
	writeJSON(w, 200, map[string]any{"items": items, "nextCursor": next, "nextCursorId": nextID})
}
func (a *API) runs(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, s.Runs)
}
func (a *API) records(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, s.Records)
}
func (a *API) preview(w http.ResponseWriter, r *http.Request) {
	var req application.CommandRequest
	if !decode(w, r, &req) {
		return
	}
	if state, ok := authState(r); ok {
		req.Principal = &application.CommandPrincipal{AccountID: state.Principal.AccountID, CredentialID: firstNonEmpty(state.Principal.CredentialID, state.Principal.SessionID), WorkspaceID: state.Principal.WorkspaceID, ApprovalActorID: state.Principal.ApprovalActorID, Subject: state.Principal.Subject}
		req.Envelope.ExecutedByActorID = state.Principal.ActorID
		req.Envelope.InitiatedByActorID = state.Principal.ActorID
		workspaceID := commandWorkspaceID(req.Arguments)
		if workspaceID == "" || state.Principal.WorkspaceID != "" && state.Principal.WorkspaceID != workspaceID {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "workspace not found"}})
			return
		}
		membership, membershipErr := a.Repo.Membership(r.Context(), workspaceID, state.Principal.ActorID)
		readDecision := authz.Authorize(authz.AuthorizationInput{Subject: state.Principal.Subject, Membership: membership, WorkspaceID: workspaceID, EntityWorkspaceID: workspaceID, Capability: authz.WorkspaceRead})
		if membershipErr != nil || !readDecision.Allowed {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "workspace not found"}})
			return
		}
	}
	v, err := a.Service.Preview(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	if state, ok := authState(r); ok {
		membership, membershipErr := a.Repo.Membership(r.Context(), commandWorkspaceID(req.Arguments), state.Principal.ActorID)
		humanDecision := false
		for _, diagnostic := range v.Errors {
			if diagnostic.Code == domain.CodeHumanApprovalRequired {
				humanDecision = true
			}
		}
		required := authz.Capability(v.RequiredCapability)
		if !humanDecision {
			decision := authz.Authorize(authz.AuthorizationInput{Subject: state.Principal.Subject, Membership: membership, WorkspaceID: commandWorkspaceID(req.Arguments), EntityWorkspaceID: commandWorkspaceID(req.Arguments), Capability: required})
			if membershipErr != nil || !decision.Allowed {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "preview capability denied"}})
				return
			}
		}
	}
	writeJSON(w, 200, v)
}
func (a *API) execute(w http.ResponseWriter, r *http.Request) {
	var req application.CommandRequest
	raw, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if readErr != nil {
		a.recordRejectedTransportAttempt(r, req, raw)
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": readErr.Error()}})
		return
	}
	if err := decodeStrict(raw, &req); err != nil {
		if state, ok := authState(r); ok {
			req.Envelope.ExecutedByActorID = state.Principal.ActorID
			req.Envelope.InitiatedByActorID = state.Principal.ActorID
		}
		a.recordRejectedTransportAttempt(r, req, raw)
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if state, ok := authState(r); ok {
		req.Principal = &application.CommandPrincipal{AccountID: state.Principal.AccountID, CredentialID: firstNonEmpty(state.Principal.CredentialID, state.Principal.SessionID), WorkspaceID: state.Principal.WorkspaceID, ApprovalActorID: state.Principal.ApprovalActorID, Subject: state.Principal.Subject}
		req.Envelope.ExecutedByActorID = state.Principal.ActorID
		req.Envelope.InitiatedByActorID = state.Principal.ActorID
		workspaceID := commandWorkspaceID(req.Arguments)
		if !a.authorizeCommandTenant(r, state, workspaceID) {
			_ = a.Repo.RecordAccessDenial(r.Context(), state.Principal.ActorID, "http.command.execute", workspaceID)
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "workspace not found"}})
			return
		}
		req.TenantAuditAuthorized = true
	}
	v, err := a.Service.Execute(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func commandWorkspaceID(arguments json.RawMessage) string {
	var value struct {
		WorkspaceID string `json:"workspaceId"`
	}
	_ = json.Unmarshal(arguments, &value)
	return strings.TrimSpace(value.WorkspaceID)
}

func (a *API) authorizeCommandTenant(r *http.Request, state authContext, workspaceID string) bool {
	if strings.TrimSpace(workspaceID) == "" || state.Principal.WorkspaceID != "" && state.Principal.WorkspaceID != workspaceID {
		return false
	}
	membership, err := a.Repo.Membership(r.Context(), workspaceID, state.Principal.ActorID)
	if err != nil {
		return false
	}
	decision := authz.Authorize(authz.AuthorizationInput{
		Subject: state.Principal.Subject, Membership: membership,
		WorkspaceID: workspaceID, EntityWorkspaceID: workspaceID, Capability: authz.WorkspaceRead,
	})
	return decision.Allowed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func projectDecisions(s application.Snapshot) []map[string]any {
	out := []map[string]any{}
	for _, t := range s.Tasks {
		if t.Status == "implemented" {
			out = append(out, map[string]any{"action": "task.confirm", "entityType": "task", "entityId": t.PublicID, "expectedWorkspaceRevision": s.Workspace.Revision})
		}
	}
	for _, g := range s.Gates {
		if g.Status == "ready" {
			out = append(out, map[string]any{"action": "gate.pass", "entityType": "gate", "entityId": g.ID, "expectedWorkspaceRevision": s.Workspace.Revision, "decisionSnapshotHash": application.DecisionSnapshotHash(s, g)})
		}
	}
	return out
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": err.Error()}})
		return false
	}
	if err = decodeStrict(raw, v); err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": err.Error()}})
		return false
	}
	return true
}
func decodeStrict(raw []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return err
	}
	return nil
}
func (a *API) recordRejectedTransportAttempt(r *http.Request, req application.CommandRequest, raw []byte) {
	var arguments struct {
		WorkspaceID string `json:"workspaceId"`
	}
	_ = json.Unmarshal(req.Arguments, &arguments)
	if state, ok := authState(r); ok {
		req.Envelope.ExecutedByActorID = state.Principal.ActorID
		req.Envelope.InitiatedByActorID = state.Principal.ActorID
	}
	if strings.TrimSpace(arguments.WorkspaceID) == "" {
		arguments.WorkspaceID = extractJSONStringField(raw, "workspaceId")
	}
	if strings.TrimSpace(arguments.WorkspaceID) == "" {
		return
	}
	if state, ok := authState(r); ok && !a.authorizeCommandTenant(r, state, arguments.WorkspaceID) {
		_ = a.Repo.RecordAccessDenial(r.Context(), state.Principal.ActorID, "http.command.execute", arguments.WorkspaceID)
		return
	}
	if req.Name == "" {
		req.Name = extractJSONStringField(raw, "name")
	}
	if req.Envelope.IdempotencyKey == "" {
		req.Envelope.IdempotencyKey = extractJSONStringField(raw, "idempotencyKey")
	}
	if req.Envelope.InitiatedByActorID == "" {
		req.Envelope.InitiatedByActorID = extractJSONStringField(raw, "initiatedByActorId")
	}
	if req.Envelope.ExecutedByActorID == "" {
		req.Envelope.ExecutedByActorID = extractJSONStringField(raw, "executedByActorId")
	}
	if req.Envelope.ExpectedWorkspaceRevision == 0 {
		req.Envelope.ExpectedWorkspaceRevision = extractJSONIntField(raw, "expectedWorkspaceRevision")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return
	}
	idempotencyHash := sha256.Sum256([]byte(req.Envelope.IdempotencyKey))
	argumentHash := sha256.Sum256(raw)
	commandName := strings.TrimSpace(req.Name)
	if commandName == "" {
		commandName = "http.command.execute"
	}
	_ = a.Repo.RecordMutationAttempt(r.Context(), application.MutationAttemptProjection{
		ID: hex.EncodeToString(random), WorkspaceID: arguments.WorkspaceID,
		CommandName: commandName, Source: "command_service", Outcome: "rejected",
		InitiatedByActorID: req.Envelope.InitiatedByActorID, ExecutedByActorID: req.Envelope.ExecutedByActorID,
		IdempotencyKeyHash: hex.EncodeToString(idempotencyHash[:]), ArgumentDigest: hex.EncodeToString(argumentHash[:]),
		ExpectedWorkspaceRevision: req.Envelope.ExpectedWorkspaceRevision,
		DiagnosticCodes:           []string{"invalid_request"}, EventIDs: []string{}, OccurredAt: time.Now().UTC(),
	})
}

func extractJSONStringField(raw []byte, field string) string {
	offset := bytes.Index(raw, []byte(`"`+field+`"`))
	if offset < 0 {
		return ""
	}
	rest := raw[offset+len(field)+2:]
	colon := bytes.IndexByte(rest, ':')
	if colon < 0 {
		return ""
	}
	var value string
	if err := json.NewDecoder(bytes.NewReader(rest[colon+1:])).Decode(&value); err != nil {
		return ""
	}
	return value
}

func extractJSONIntField(raw []byte, field string) int64 {
	offset := bytes.Index(raw, []byte(`"`+field+`"`))
	if offset < 0 {
		return 0
	}
	rest := raw[offset+len(field)+2:]
	colon := bytes.IndexByte(rest, ':')
	if colon < 0 {
		return 0
	}
	var value int64
	if err := json.NewDecoder(bytes.NewReader(rest[colon+1:])).Decode(&value); err != nil {
		return 0
	}
	return value
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	var ce *application.CommandError
	if !errors.As(err, &ce) {
		writeJSON(w, 500, map[string]any{"error": map[string]string{"code": "internal_error", "message": "internal server error"}})
		return
	}
	status := 422
	switch ce.Code {
	case "not_found":
		status = 404
	case "stale_revision", "stale_run_version", "run_lease_mismatch", "idempotency_conflict", "invalid_state_transition", "gate_not_ready", "gate_not_current":
		status = 409
	case "invalid_request":
		status = 400
	case "invalid_backlog_filter":
		status = 400
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": ce.Code, "message": ce.Message}})
}
func (a *API) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && a.isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Baley-CSRF")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) isOriginAllowed(origin string) bool {
	for _, allowed := range a.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

type responseCapture struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseCapture) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseCapture) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(value)
	w.bytes += n
	return n, err
}

func (w *responseCapture) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (a *API) observability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := validRequestID(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = randomRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Baley-Version", a.buildVersion())
		capture := &responseCapture{ResponseWriter: w}
		next.ServeHTTP(capture, r)
		if capture.status == 0 {
			capture.status = http.StatusOK
		}
		entry, _ := json.Marshal(map[string]any{
			"event": "http_request", "requestId": requestID, "method": r.Method,
			"path": r.URL.Path, "status": capture.status, "bytes": capture.bytes,
			"durationMs": time.Since(started).Milliseconds(),
		})
		log.Print(string(entry))
	})
}

func (a *API) buildVersion() string {
	if strings.TrimSpace(a.Build.Version) == "" {
		return "dev"
	}
	return a.Build.Version
}

func validRequestID(value string) string {
	if len(value) < 8 || len(value) > 128 {
		return ""
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			continue
		}
		return ""
	}
	return value
}

func randomRequestID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err == nil {
		return hex.EncodeToString(raw)
	}
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
}
