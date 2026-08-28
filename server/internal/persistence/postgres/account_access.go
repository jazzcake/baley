package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jazzcake/baley/server/internal/authn"
	"github.com/jazzcake/baley/server/internal/authz"
)

type WorkspaceAccess struct {
	ID, Name, State, Role string
	Revision              int64
	Capabilities          []authz.Capability
	Idempotent            bool
}

type MemberAccess struct {
	ActorID, AccountID, DisplayName, Role string
	Active                                bool
}

type AgentTokenResult struct {
	ID, Token, Prefix string
}

type CreatedMember struct {
	AccountID, ActorID string
}

func (r *Repository) CreateOwnedWorkspace(ctx context.Context, workspaceID, name, creatorActorID string) (WorkspaceAccess, error) {
	workspaceID, name, creatorActorID = strings.TrimSpace(workspaceID), strings.TrimSpace(name), strings.TrimSpace(creatorActorID)
	if workspaceID == "" || name == "" || len([]rune(name)) > 120 || creatorActorID == "" {
		return WorkspaceAccess{}, fmt.Errorf("invalid Workspace identity")
	}
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return WorkspaceAccess{}, err
	}
	defer tx.Rollback(ctx)
	var accountActive bool
	if err = tx.QueryRow(ctx, `SELECT account.status='active'
		FROM accounts account
		JOIN actors actor ON actor.id=account.actor_id AND actor.actor_type='human'
		WHERE account.actor_id=$1 FOR SHARE`, creatorActorID).Scan(&accountActive); err != nil || !accountActive {
		return WorkspaceAccess{}, fmt.Errorf("active human account required")
	}
	// The client-generated Workspace UUID is the retry identity. Replay only an
	// exact create owned by the same active human and evidenced by the original
	// workspace.created security Event; pre-existing seeded/imported Workspaces
	// must continue to conflict.
	var existing WorkspaceAccess
	var existingActive bool
	err = tx.QueryRow(ctx, `SELECT workspace.id,workspace.name,workspace.state,workspace.revision,
		membership.role,membership.active
		FROM workspaces workspace
		JOIN workspace_memberships membership
		  ON membership.workspace_id=workspace.id AND membership.actor_id=$2
		WHERE workspace.id=$1 AND membership.created_by_actor_id=$2
		  AND EXISTS (
		    SELECT 1 FROM security_events creation
		    WHERE creation.workspace_id=workspace.id
		      AND creation.actor_id=$2
		      AND creation.event_type='workspace.created'
		      AND creation.entity_type='workspace'
		      AND creation.entity_id=workspace.id)
		FOR SHARE OF workspace,membership`, workspaceID, creatorActorID).
		Scan(&existing.ID, &existing.Name, &existing.State, &existing.Revision, &existing.Role, &existingActive)
	if err == nil {
		if existing.Name != name || existing.State != "active" || existing.Role != string(authz.RoleOwner) || !existingActive {
			return WorkspaceAccess{}, fmt.Errorf("Workspace identity is already in use")
		}
		existing.Capabilities, err = authz.ResolveRole(authz.RoleOwner, authz.ActorHuman, authz.HumanSession)
		if err != nil {
			return WorkspaceAccess{}, err
		}
		existing.Idempotent = true
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceAccess{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspaces(id,name,state,revision)
		VALUES($1,$2,'active',1)`, workspaceID, name); err != nil {
		return WorkspaceAccess{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspace_memberships(
		workspace_id,actor_id,role,active,created_by_actor_id)
		VALUES($1,$2,'owner',true,$2)`, workspaceID, creatorActorID); err != nil {
		return WorkspaceAccess{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO phases(workspace_id,id,name,position,state)
		VALUES($1,'intake','Intake',1,'active')`, workspaceID); err != nil {
		return WorkspaceAccess{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO lanes(workspace_id,id,name,goal,summary,state)
		VALUES($1,'adoption','Adoption','','','active')`, workspaceID); err != nil {
		return WorkspaceAccess{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspace_counters(
		workspace_id,next_task_public_id,next_backlog_public_id,next_gate_public_id)
		VALUES($1,1,1,1)`, workspaceID); err != nil {
		return WorkspaceAccess{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE workspace_acceptance_policies
		SET changed_by_actor_id=$2
		WHERE workspace_id=$1 AND default_mode='human_required'`, workspaceID, creatorActorID); err != nil {
		return WorkspaceAccess{}, err
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, "", creatorActorID,
		"workspace.created", "workspace", workspaceID,
		map[string]any{"initialPhaseId": "intake", "initialLaneId": "adoption", "ownerActorId": creatorActorID}); err != nil {
		return WorkspaceAccess{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkspaceAccess{}, err
	}
	capabilities, err := authz.ResolveRole(authz.RoleOwner, authz.ActorHuman, authz.HumanSession)
	if err != nil {
		return WorkspaceAccess{}, err
	}
	return WorkspaceAccess{ID: workspaceID, Name: name, State: "active", Role: string(authz.RoleOwner), Revision: 1, Capabilities: capabilities}, nil
}

func (r *Repository) AccountCredentialByLogin(ctx context.Context, normalized string) (authn.AccountCredential, error) {
	var value authn.AccountCredential
	err := r.Pool.QueryRow(ctx, `SELECT account.id::text,account.actor_id,account.login_id,account.display_name,
		account.status,credential.password_phc
		FROM accounts account
		JOIN account_credentials credential ON credential.account_id=account.id
		WHERE account.normalized_login_id=$1`, normalized).
		Scan(&value.AccountID, &value.ActorID, &value.LoginID, &value.DisplayName, &value.Status, &value.PasswordPHC)
	return value, err
}

func (r *Repository) AccountCredentialByID(ctx context.Context, accountID string) (authn.AccountCredential, error) {
	var value authn.AccountCredential
	err := r.Pool.QueryRow(ctx, `SELECT account.id::text,account.actor_id,account.login_id,account.display_name,
		account.status,credential.password_phc
		FROM accounts account JOIN account_credentials credential ON credential.account_id=account.id
		WHERE account.id=$1`, accountID).
		Scan(&value.AccountID, &value.ActorID, &value.LoginID, &value.DisplayName, &value.Status, &value.PasswordPHC)
	return value, err
}

func (r *Repository) LoginRateLimited(ctx context.Context, key []byte, now time.Time) (bool, error) {
	var blocked *time.Time
	err := r.Pool.QueryRow(ctx, "SELECT blocked_until FROM auth_login_limits WHERE key_hash=$1", key).Scan(&blocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil && blocked != nil && now.Before(*blocked), err
}

func (r *Repository) RecordLoginFailure(ctx context.Context, key []byte, now time.Time) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO auth_login_limits(key_hash,window_started_at,failure_count,blocked_until)
		VALUES($1,$2,1,NULL)
		ON CONFLICT(key_hash) DO UPDATE SET
		  window_started_at=CASE WHEN auth_login_limits.window_started_at < $2-interval '15 minutes' THEN $2 ELSE auth_login_limits.window_started_at END,
		  failure_count=CASE WHEN auth_login_limits.window_started_at < $2-interval '15 minutes' THEN 1 ELSE auth_login_limits.failure_count+1 END,
		  blocked_until=CASE
		    WHEN (CASE WHEN auth_login_limits.window_started_at < $2-interval '15 minutes' THEN 1 ELSE auth_login_limits.failure_count+1 END) >= 5
		    THEN $2+interval '15 minutes' ELSE auth_login_limits.blocked_until END`, key, now)
	return err
}

func (r *Repository) RecordAuthenticationFailure(ctx context.Context, loginDigest string, now time.Time) error {
	payload, _ := json.Marshal(map[string]any{"outcome": "rejected"})
	_, err := r.Pool.Exec(ctx, `INSERT INTO security_events(id,event_type,entity_type,entity_id,payload,created_at)
		VALUES($1,'authentication.login_failed','login_digest',$2,$3,$4)`, newID(), loginDigest, payload, now)
	return err
}

func (r *Repository) RecordAccessDenial(ctx context.Context, actorID, route, targetWorkspaceID string) error {
	workspaceDigest := sha256.Sum256([]byte(strings.TrimSpace(targetWorkspaceID)))
	payload, err := json.Marshal(map[string]any{"route": strings.TrimSpace(route)})
	if err != nil {
		return err
	}
	_, err = r.Pool.Exec(ctx, `INSERT INTO security_events(
		id,workspace_id,actor_id,event_type,entity_type,entity_id,payload)
		VALUES($1,NULL,$2,'authorization.workspace_denied','workspace_digest',$3,$4)`,
		newID(), actorID, base64.RawURLEncoding.EncodeToString(workspaceDigest[:]), payload)
	return err
}

func (r *Repository) ClearLoginFailures(ctx context.Context, key []byte) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM auth_login_limits WHERE key_hash=$1", key)
	return err
}

func (r *Repository) CreateSession(ctx context.Context, value authn.NewSession) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO account_sessions(
		id,account_id,token_hash,csrf_token_hash,created_at,last_seen_at,idle_expires_at,absolute_expires_at)
		VALUES($1,$2,$3,$4,$5,$5,$6,$7)`,
		value.ID, value.AccountID, value.TokenHash, value.CSRFTokenHash, value.CreatedAt, value.IdleExpiresAt, value.AbsoluteAt)
	if err == nil {
		var actorID string
		err = tx.QueryRow(ctx, "SELECT actor_id FROM accounts WHERE id=$1", value.AccountID).Scan(&actorID)
		if err == nil {
			err = insertSecurityEvent(ctx, tx, "", value.AccountID, actorID, "authentication.login_succeeded", "session", value.ID, map[string]any{"expiresAt": value.AbsoluteAt})
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) SessionByTokenHash(ctx context.Context, hash []byte) (authn.SessionRecord, error) {
	var value authn.SessionRecord
	err := r.Pool.QueryRow(ctx, `SELECT session.id::text,session.account_id::text,account.actor_id,
		account.login_id,account.display_name,account.status,session.token_hash,session.csrf_token_hash,
		session.last_seen_at,session.idle_expires_at,session.absolute_expires_at,session.revoked_at
		FROM account_sessions session JOIN accounts account ON account.id=session.account_id
		WHERE session.token_hash=$1`, hash).
		Scan(&value.ID, &value.AccountID, &value.ActorID, &value.LoginID, &value.DisplayName,
			&value.AccountStatus, &value.TokenHash, &value.CSRFTokenHash, &value.LastSeenAt,
			&value.IdleExpiresAt, &value.AbsoluteExpiresAt, &value.RevokedAt)
	return value, err
}

func (r *Repository) TouchSession(ctx context.Context, id string, seen, idle time.Time) error {
	_, err := r.Pool.Exec(ctx, "UPDATE account_sessions SET last_seen_at=$1,idle_expires_at=$2 WHERE id=$3 AND revoked_at IS NULL", seen, idle, id)
	return err
}

func (r *Repository) RevokeSession(ctx context.Context, id string, now time.Time) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var accountID, actorID string
	if err = tx.QueryRow(ctx, `SELECT session.account_id::text,account.actor_id
		FROM account_sessions session JOIN accounts account ON account.id=session.account_id
		WHERE session.id=$1 FOR UPDATE`, id).Scan(&accountID, &actorID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE account_sessions SET revoked_at=COALESCE(revoked_at,$1) WHERE id=$2", now, id); err == nil {
		err = insertSecurityEvent(ctx, tx, "", accountID, actorID, "authentication.logged_out", "session", id, map[string]any{})
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ReplacePassword(ctx context.Context, accountID, passwordPHC string, now time.Time) error {
	_, err := r.Pool.Exec(ctx, `UPDATE account_credentials
		SET password_phc=$1,credential_version=credential_version+1,password_changed_at=$2
		WHERE account_id=$3`, passwordPHC, now, accountID)
	return err
}

func (r *Repository) RevokeAccountSessions(ctx context.Context, accountID string, now time.Time) error {
	_, err := r.Pool.Exec(ctx, "UPDATE account_sessions SET revoked_at=COALESCE(revoked_at,$1) WHERE account_id=$2", now, accountID)
	return err
}

func (r *Repository) ChangePasswordAndRevokeSessions(ctx context.Context, accountID, passwordPHC string, now time.Time) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE account_credentials
		SET password_phc=$1,credential_version=credential_version+1,password_changed_at=$2
		WHERE account_id=$3`, passwordPHC, now, accountID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE account_sessions SET revoked_at=COALESCE(revoked_at,$1) WHERE account_id=$2", now, accountID); err != nil {
		return err
	}
	var actorID string
	if err = tx.QueryRow(ctx, "SELECT actor_id FROM accounts WHERE id=$1", accountID).Scan(&actorID); err == nil {
		err = insertSecurityEvent(ctx, tx, "", accountID, actorID, "authentication.password_changed", "account", accountID, map[string]any{"sessionsRevoked": true})
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) AgentByTokenHash(ctx context.Context, hash []byte, now time.Time) (authn.AgentTokenRecord, error) {
	var value authn.AgentTokenRecord
	var raw []byte
	err := r.Pool.QueryRow(ctx, `SELECT token.id::text,token.actor_id,token.workspace_id,token.created_by_actor_id,token.scopes
		FROM agent_tokens token
		JOIN actors actor ON actor.id=token.actor_id AND actor.actor_type='agent'
		JOIN workspace_memberships membership
		  ON membership.workspace_id=token.workspace_id AND membership.actor_id=token.actor_id
		  AND membership.active AND membership.role='operator'
		LEFT JOIN mcp_gateway_registrations gateway ON gateway.id=token.gateway_registration_id
		LEFT JOIN workspace_memberships gateway_member ON gateway_member.workspace_id=gateway.workspace_id
		  AND gateway_member.actor_id=gateway.account_actor_id AND gateway_member.active
		LEFT JOIN accounts gateway_account ON gateway_account.actor_id=gateway.account_actor_id AND gateway_account.status='active'
		WHERE token.token_hash=$1 AND token.revoked_at IS NULL
		  AND (token.gateway_registration_id IS NULL OR (gateway.status='active' AND gateway.agent_actor_id=token.actor_id AND gateway_member.actor_id IS NOT NULL AND gateway_account.actor_id IS NOT NULL))
		  AND (token.expires_at IS NULL OR token.expires_at>$2)`, hash, now).
		Scan(&value.TokenID, &value.ActorID, &value.WorkspaceID, &value.CreatedByActorID, &raw)
	if err != nil {
		return value, err
	}
	if err = json.Unmarshal(raw, &value.Scopes); err != nil {
		return value, err
	}
	for _, scope := range value.Scopes {
		if scope == authz.TaskApprove || scope == authz.LaneApprove || scope == authz.GateApprove || scope == authz.WorkspaceClose || scope == authz.WorkspaceAdmin {
			return authn.AgentTokenRecord{}, authz.ErrRoleForbiddenForActor
		}
	}
	_, _ = r.Pool.Exec(ctx, "UPDATE agent_tokens SET last_used_at=$1 WHERE id=$2 AND (last_used_at IS NULL OR last_used_at<$1-interval '1 minute')", now, value.TokenID)
	return value, nil
}

func (r *Repository) Membership(ctx context.Context, workspaceID, actorID string) (*authz.Membership, error) {
	value := &authz.Membership{ActorID: actorID, WorkspaceID: workspaceID}
	var role string
	err := r.Pool.QueryRow(ctx, "SELECT role,active FROM workspace_memberships WHERE workspace_id=$1 AND actor_id=$2", workspaceID, actorID).Scan(&role, &value.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	value.Role = authz.Role(role)
	return value, err
}

func (r *Repository) ListAccountWorkspaces(ctx context.Context, accountID string) ([]WorkspaceAccess, error) {
	rows, err := r.Pool.Query(ctx, `SELECT workspace.id,workspace.name,workspace.state,workspace.revision,membership.role
		FROM accounts account
		JOIN workspace_memberships membership ON membership.actor_id=account.actor_id AND membership.active
		JOIN workspaces workspace ON workspace.id=membership.workspace_id
		WHERE account.id=$1 AND account.status='active'
		ORDER BY lower(workspace.name),workspace.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []WorkspaceAccess{}
	for rows.Next() {
		var item WorkspaceAccess
		if err = rows.Scan(&item.ID, &item.Name, &item.State, &item.Revision, &item.Role); err != nil {
			return nil, err
		}
		item.Capabilities, err = authz.ResolveRole(authz.Role(item.Role), authz.ActorHuman, authz.HumanSession)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListMembers(ctx context.Context, workspaceID string) ([]MemberAccess, error) {
	rows, err := r.Pool.Query(ctx, `SELECT membership.actor_id,COALESCE(account.id::text,''),
		COALESCE(account.display_name,actor.display_name),membership.role,membership.active
		FROM workspace_memberships membership
		JOIN actors actor ON actor.id=membership.actor_id
		LEFT JOIN accounts account ON account.actor_id=membership.actor_id
		WHERE membership.workspace_id=$1
		ORDER BY CASE membership.role WHEN 'owner' THEN 0 ELSE 1 END,
		         lower(COALESCE(account.display_name,actor.display_name)),membership.actor_id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MemberAccess{}
	for rows.Next() {
		var item MemberAccess
		if err = rows.Scan(&item.ActorID, &item.AccountID, &item.DisplayName, &item.Role, &item.Active); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) BootstrapOwner(ctx context.Context, workspaceID, accountID, actorID, loginID, normalizedLoginID, displayName, passwordPHC string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workspaces WHERE id=$1 FOR UPDATE", workspaceID); err != nil {
		return err
	}
	var actorType string
	err = tx.QueryRow(ctx, "SELECT actor_type FROM actors WHERE id=$1", actorID).Scan(&actorType)
	if err != nil || actorType != "human" {
		return errors.New("bootstrap actor must be an existing human actor")
	}
	_, err = tx.Exec(ctx, `INSERT INTO accounts(id,actor_id,login_id,normalized_login_id,display_name)
		VALUES($1,$2,$3,$4,$5)`, accountID, actorID, strings.TrimSpace(loginID), normalizedLoginID, strings.TrimSpace(displayName))
	if err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO account_credentials(account_id,password_phc) VALUES($1,$2)", accountID, passwordPHC)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id)
			VALUES($1,$2,'owner',true,$2)
			ON CONFLICT(workspace_id,actor_id) DO UPDATE SET role='owner',active=true,deactivated_at=NULL,updated_at=now()`, workspaceID, actorID)
	}
	if err == nil {
		err = insertSecurityEvent(ctx, tx, workspaceID, accountID, actorID, "workspace.owner_bootstrapped", "membership", actorID, map[string]any{"role": "owner"})
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) CreateMember(ctx context.Context, workspaceID, createdByActorID, loginID, normalizedLoginID, displayName, passwordPHC string, role authz.Role) (CreatedMember, error) {
	if role != authz.RoleViewer && role != authz.RoleOperator && role != authz.RoleApprover && role != authz.RoleOwner {
		return CreatedMember{}, authz.ErrUnknownRole
	}
	accountID, err := randomUUIDString()
	if err != nil {
		return CreatedMember{}, err
	}
	actorID, err := randomUUIDString()
	if err != nil {
		return CreatedMember{}, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return CreatedMember{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workspaces WHERE id=$1 FOR UPDATE", workspaceID); err != nil {
		return CreatedMember{}, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO actors(id,display_name,actor_type) VALUES($1,$2,'human')", actorID, strings.TrimSpace(displayName)); err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO accounts(id,actor_id,login_id,normalized_login_id,display_name)
			VALUES($1,$2,$3,$4,$5)`, accountID, actorID, strings.TrimSpace(loginID), normalizedLoginID, strings.TrimSpace(displayName))
	}
	if err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO account_credentials(account_id,password_phc) VALUES($1,$2)", accountID, passwordPHC)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id)
			VALUES($1,$2,$3,true,$4)`, workspaceID, actorID, role, createdByActorID)
	}
	if err == nil {
		err = insertSecurityEvent(ctx, tx, workspaceID, accountID, createdByActorID, "workspace.member_created", "membership", actorID, map[string]any{"role": role, "active": true})
	}
	if err != nil {
		return CreatedMember{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CreatedMember{}, err
	}
	return CreatedMember{AccountID: accountID, ActorID: actorID}, nil
}

func (r *Repository) AddExistingMember(ctx context.Context, workspaceID, createdByActorID, normalizedLoginID string, role authz.Role) (MemberAccess, error) {
	if role != authz.RoleViewer && role != authz.RoleOperator && role != authz.RoleApprover && role != authz.RoleOwner {
		return MemberAccess{}, authz.ErrUnknownRole
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return MemberAccess{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workspaces WHERE id=$1 FOR UPDATE", workspaceID); err != nil {
		return MemberAccess{}, err
	}
	var member MemberAccess
	var status string
	if err = tx.QueryRow(ctx, `SELECT account.id::text,account.actor_id,account.display_name,account.status
		FROM accounts account WHERE account.normalized_login_id=$1 FOR UPDATE`, normalizedLoginID).
		Scan(&member.AccountID, &member.ActorID, &member.DisplayName, &status); err != nil {
		return MemberAccess{}, err
	}
	if status != "active" {
		return MemberAccess{}, errors.New("disabled account cannot be added to a Workspace")
	}
	tag, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id)
		VALUES($1,$2,$3,true,$4)
		ON CONFLICT(workspace_id,actor_id) DO NOTHING`, workspaceID, member.ActorID, role, createdByActorID)
	if err != nil {
		return MemberAccess{}, err
	}
	if tag.RowsAffected() != 1 {
		return MemberAccess{}, errors.New("account is already a Workspace member")
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, "", createdByActorID, "workspace.existing_account_attached", "membership", member.ActorID, map[string]any{"role": role}); err != nil {
		return MemberAccess{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MemberAccess{}, err
	}
	member.Role, member.Active = string(role), true
	return member, nil
}

func (r *Repository) DisableMemberAccount(ctx context.Context, workspaceID, targetActorID, disabledByActorID string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workspaces WHERE id=$1 FOR UPDATE", workspaceID); err != nil {
		return err
	}
	var accountID, status string
	if err = tx.QueryRow(ctx, `SELECT account.id::text,account.status
		FROM workspace_memberships membership
		JOIN accounts account ON account.actor_id=membership.actor_id
		WHERE membership.workspace_id=$1 AND membership.actor_id=$2
		FOR UPDATE OF account`, workspaceID, targetActorID).Scan(&accountID, &status); err != nil {
		return err
	}
	if status != "active" {
		return errors.New("account is already disabled")
	}
	var otherActiveMemberships int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM workspace_memberships
		WHERE actor_id=$1 AND active AND workspace_id<>$2`, targetActorID, workspaceID).Scan(&otherActiveMemberships); err != nil {
		return err
	}
	if otherActiveMemberships > 0 {
		return errors.New("account has active memberships in other Workspaces; global disable requires system administration")
	}
	if _, err = tx.Exec(ctx, "UPDATE accounts SET status='disabled',disabled_at=now(),updated_at=now() WHERE id=$1 AND status='active'", accountID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE account_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE account_id=$1", accountID); err != nil {
		return err
	}
	if err = r.revokeMCPGatewaysForMemberTx(ctx, tx, workspaceID, targetActorID, disabledByActorID, "account_disabled", time.Now().UTC()); err != nil {
		return err
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, accountID, disabledByActorID, "account.disabled", "account", accountID, map[string]any{"sessionsRevoked": true}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) AdminResetMemberPassword(ctx context.Context, workspaceID, targetActorID, resetByActorID, passwordPHC string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workspaces WHERE id=$1 FOR UPDATE", workspaceID); err != nil {
		return err
	}
	var accountID, status string
	if err = tx.QueryRow(ctx, `SELECT account.id::text,account.status
		FROM workspace_memberships membership
		JOIN accounts account ON account.actor_id=membership.actor_id
		WHERE membership.workspace_id=$1 AND membership.actor_id=$2
		FOR UPDATE OF account`, workspaceID, targetActorID).Scan(&accountID, &status); err != nil {
		return err
	}
	if status != "active" {
		return errors.New("disabled account password cannot be reset")
	}
	var otherActiveMemberships int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM workspace_memberships
		WHERE actor_id=$1 AND active AND workspace_id<>$2`, targetActorID, workspaceID).Scan(&otherActiveMemberships); err != nil {
		return err
	}
	if otherActiveMemberships > 0 {
		return errors.New("account has active memberships in other Workspaces; global password reset requires system administration")
	}
	tag, err := tx.Exec(ctx, `UPDATE account_credentials
		SET password_phc=$1,credential_version=credential_version+1,password_changed_at=now()
		WHERE account_id=$2`, passwordPHC, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	if _, err = tx.Exec(ctx, "UPDATE account_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE account_id=$1", accountID); err != nil {
		return err
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, accountID, resetByActorID, "account.password_reset", "account", accountID, map[string]any{"sessionsRevoked": true}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) UpdateMember(ctx context.Context, workspaceID, actorID, changedByActorID string, role *authz.Role, active *bool) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workspaces WHERE id=$1 FOR UPDATE", workspaceID); err != nil {
		return err
	}
	changed := false
	if role != nil {
		if *role != authz.RoleViewer && *role != authz.RoleOperator && *role != authz.RoleApprover && *role != authz.RoleOwner {
			return authz.ErrUnknownRole
		}
		tag, updateErr := tx.Exec(ctx, "UPDATE workspace_memberships SET role=$1,updated_at=now() WHERE workspace_id=$2 AND actor_id=$3", *role, workspaceID, actorID)
		err = updateErr
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
		changed = true
	}
	if active != nil {
		tag, updateErr := tx.Exec(ctx, `UPDATE workspace_memberships
			SET active=$1,deactivated_at=CASE WHEN $1 THEN NULL ELSE now() END,updated_at=now()
			WHERE workspace_id=$2 AND actor_id=$3`, *active, workspaceID, actorID)
		err = updateErr
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
		changed = true
	}
	if !changed {
		return errors.New("membership update requires role or active")
	}
	// Gateway credentials are derived from this human membership. Revoke them
	// atomically with every role or active-state change.
	if err = r.revokeMCPGatewaysForMemberTx(ctx, tx, workspaceID, actorID, changedByActorID, "membership_changed", time.Now().UTC()); err != nil {
		return err
	}
	eventType := "workspace.member_updated"
	if active != nil && !*active {
		eventType = "workspace.member_deactivated"
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, "", changedByActorID, eventType, "membership", actorID, map[string]any{"role": role, "active": active}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) TransferOwner(ctx context.Context, workspaceID, sourceActorID, targetActorID string, previousOwnerRole authz.Role) error {
	if previousOwnerRole != authz.RoleViewer && previousOwnerRole != authz.RoleOperator && previousOwnerRole != authz.RoleApprover {
		return authz.ErrUnknownRole
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workspaces WHERE id=$1 FOR UPDATE", workspaceID); err != nil {
		return err
	}
	var sourceRole, targetRole string
	var sourceActive, targetActive bool
	if err = tx.QueryRow(ctx, "SELECT role,active FROM workspace_memberships WHERE workspace_id=$1 AND actor_id=$2 FOR UPDATE", workspaceID, sourceActorID).Scan(&sourceRole, &sourceActive); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, "SELECT role,active FROM workspace_memberships WHERE workspace_id=$1 AND actor_id=$2 FOR UPDATE", workspaceID, targetActorID).Scan(&targetRole, &targetActive); err != nil {
		return err
	}
	if sourceRole != string(authz.RoleOwner) || !sourceActive || !targetActive {
		return errors.New("Owner transfer requires an active source Owner and active target member")
	}
	if _, err = tx.Exec(ctx, "UPDATE workspace_memberships SET role='owner',updated_at=now() WHERE workspace_id=$1 AND actor_id=$2", workspaceID, targetActorID); err == nil {
		_, err = tx.Exec(ctx, "UPDATE workspace_memberships SET role=$1,updated_at=now() WHERE workspace_id=$2 AND actor_id=$3", previousOwnerRole, workspaceID, sourceActorID)
	}
	if err == nil {
		err = r.revokeMCPGatewaysForMemberTx(ctx, tx, workspaceID, sourceActorID, sourceActorID, "membership_changed", time.Now().UTC())
	}
	if err == nil {
		err = r.revokeMCPGatewaysForMemberTx(ctx, tx, workspaceID, targetActorID, sourceActorID, "membership_changed", time.Now().UTC())
	}
	if err == nil {
		err = insertSecurityEvent(ctx, tx, workspaceID, "", sourceActorID, "workspace.owner_transferred", "membership", targetActorID, map[string]any{"previousOwnerActorId": sourceActorID, "previousOwnerRole": previousOwnerRole})
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func DigestSecret(value string) []byte {
	hash := sha256.Sum256([]byte(value))
	return hash[:]
}

func (r *Repository) IssueAgentToken(ctx context.Context, workspaceID, actorID, name, createdByActorID string, scopes []authz.Capability, expiresAt *time.Time) (AgentTokenResult, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return AgentTokenResult{}, err
	}
	defer tx.Rollback(ctx)
	result, err := r.issueAgentTokenTx(ctx, tx, workspaceID, actorID, name, createdByActorID, scopes, expiresAt, nil)
	if err != nil {
		return AgentTokenResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AgentTokenResult{}, err
	}
	return result, nil
}

func (r *Repository) issueAgentTokenTx(ctx context.Context, tx pgx.Tx, workspaceID, actorID, name, createdByActorID string, scopes []authz.Capability, expiresAt *time.Time, gatewayRegistrationID *string) (AgentTokenResult, error) {
	allowed := map[authz.Capability]bool{}
	for _, capability := range authz.DefaultCatalog.Roles[authz.RoleOperator] {
		allowed[capability] = true
	}
	if len(scopes) == 0 {
		scopes = append([]authz.Capability(nil), authz.DefaultCatalog.Roles[authz.RoleOperator]...)
	}
	for _, scope := range scopes {
		if !allowed[scope] {
			return AgentTokenResult{}, authz.ErrRoleForbiddenForActor
		}
	}
	token, hash, err := randomOpaqueSecret()
	if err != nil {
		return AgentTokenResult{}, err
	}
	id, err := randomUUIDString()
	if err != nil {
		return AgentTokenResult{}, err
	}
	prefix := token
	if len(prefix) > 10 {
		prefix = prefix[:10]
	}
	rawScopes, _ := json.Marshal(scopes)
	var kind string
	if err = tx.QueryRow(ctx, "SELECT actor_type FROM actors WHERE id=$1", actorID).Scan(&kind); err != nil || kind != "agent" {
		return AgentTokenResult{}, errors.New("Agent token requires an Agent actor")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id)
		VALUES($1,$2,'operator',true,$3)
		ON CONFLICT(workspace_id,actor_id) DO UPDATE SET role='operator',active=true,deactivated_at=NULL,updated_at=now()`,
		workspaceID, actorID, createdByActorID); err != nil {
		return AgentTokenResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO agent_tokens(id,workspace_id,actor_id,name,token_prefix,token_hash,scopes,created_by_actor_id,expires_at,gateway_registration_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, workspaceID, actorID, strings.TrimSpace(name), prefix, hash, rawScopes, createdByActorID, expiresAt, gatewayRegistrationID); err != nil {
		return AgentTokenResult{}, err
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, "", createdByActorID, "agent_token.issued", "agent_token", id, map[string]any{"agentActorId": actorID, "name": strings.TrimSpace(name), "scopes": scopes, "expiresAt": expiresAt}); err != nil {
		return AgentTokenResult{}, err
	}
	return AgentTokenResult{ID: id, Token: token, Prefix: prefix}, nil
}

func (r *Repository) RevokeAgentToken(ctx context.Context, workspaceID, tokenID, revokedByActorID string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE agent_tokens SET revoked_at=now()
		WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`, tokenID, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, "", revokedByActorID, "agent_token.revoked", "agent_token", tokenID, map[string]any{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ValidateEnforcedOwners(ctx context.Context) error {
	var count int
	err := r.Pool.QueryRow(ctx, `SELECT count(*)
		FROM workspaces workspace
		WHERE workspace.state='active'
		  AND NOT EXISTS (
		    SELECT 1 FROM workspace_memberships membership
		    JOIN actors actor ON actor.id=membership.actor_id AND actor.actor_type='human'
		    JOIN accounts account ON account.actor_id=membership.actor_id AND account.status='active'
		    WHERE membership.workspace_id=workspace.id AND membership.active AND membership.role='owner'
		  )`).Scan(&count)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("%d active Workspaces have no active account-linked Owner; run account bootstrap before enforced mode", count)
	}
	return nil
}

func randomOpaqueSecret() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, DigestSecret(token), nil
}

func randomUUIDString() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

func insertSecurityEvent(ctx context.Context, tx pgx.Tx, workspaceID, accountID, actorID, eventType, entityType, entityID string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO security_events(
		id,workspace_id,account_id,actor_id,event_type,entity_type,entity_id,payload)
		VALUES($1,NULLIF($2,''),NULLIF($3,'')::uuid,NULLIF($4,''),$5,$6,$7,$8)`,
		newID(), workspaceID, accountID, actorID, eventType, entityType, entityID, raw)
	return err
}
