package authz

import (
	"sort"
)

type Capability string

const (
	WorkspaceRead    Capability = "workspace:read"
	WorkspaceOperate Capability = "workspace:operate"
	RunOperate       Capability = "run:operate"
	RecordOperate    Capability = "record:operate"
	TaskApprove      Capability = "task:approve"
	LaneApprove      Capability = "lane:approve"
	GateApprove      Capability = "gate:approve"
	WorkspaceClose   Capability = "workspace:close"
	WorkspaceAdmin   Capability = "workspace:admin"
)

var Capabilities = []Capability{WorkspaceRead, WorkspaceOperate, RunOperate, RecordOperate, TaskApprove, LaneApprove, GateApprove, WorkspaceClose, WorkspaceAdmin}
var IntrinsicAgentForbidden = []Capability{TaskApprove, LaneApprove, GateApprove, WorkspaceClose, WorkspaceAdmin}

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleApprover Role = "approver"
	RoleOwner    Role = "owner"
)

var Roles = []Role{RoleViewer, RoleOperator, RoleApprover, RoleOwner}

type ActorKind string

const (
	ActorHuman  ActorKind = "human"
	ActorAgent  ActorKind = "agent"
	ActorSystem ActorKind = "system"
)

type CredentialKind string

const (
	HumanSession     CredentialKind = "human_session"
	AgentToken       CredentialKind = "agent_token"
	SystemCredential CredentialKind = "system_credential"
)

type Catalog struct {
	Capabilities     []Capability
	Roles            map[Role][]Capability
	AgentAllowedRole Role
	AgentForbidden   []Capability
}

var DefaultCatalog = Catalog{
	Capabilities: append([]Capability(nil), Capabilities...),
	Roles: map[Role][]Capability{
		RoleViewer:   {WorkspaceRead},
		RoleOperator: {WorkspaceRead, WorkspaceOperate, RunOperate, RecordOperate},
		RoleApprover: {WorkspaceRead, TaskApprove, LaneApprove, GateApprove},
		RoleOwner:    append([]Capability(nil), Capabilities...),
	},
	AgentAllowedRole: RoleOperator,
	AgentForbidden:   append([]Capability(nil), IntrinsicAgentForbidden...),
}

func ValidateCatalog(catalog Catalog) error {
	known := map[Capability]bool{}
	for _, capability := range catalog.Capabilities {
		if capability == "" || known[capability] {
			return ErrInvalidCatalog
		}
		known[capability] = true
	}
	if len(known) == 0 || catalog.AgentAllowedRole != RoleOperator {
		return ErrInvalidCatalog
	}
	for _, role := range Roles {
		values, exists := catalog.Roles[role]
		if !exists || len(values) == 0 {
			return ErrInvalidCatalog
		}
		seen := map[Capability]bool{}
		for _, capability := range values {
			if !known[capability] || seen[capability] {
				return ErrInvalidCatalog
			}
			seen[capability] = true
		}
	}
	if len(catalog.Roles) != len(Roles) {
		return ErrInvalidCatalog
	}
	operator := capabilitySet(catalog.Roles[catalog.AgentAllowedRole])
	seenForbidden := map[Capability]bool{}
	for _, capability := range catalog.AgentForbidden {
		if !known[capability] || operator[capability] || seenForbidden[capability] {
			return ErrInvalidCatalog
		}
		seenForbidden[capability] = true
	}
	if !sameCapabilitySet(catalog.AgentForbidden, IntrinsicAgentForbidden) {
		return ErrInvalidCatalog
	}
	return nil
}

func ResolveRole(role Role, actorKind ActorKind, credential CredentialKind) ([]Capability, error) {
	if ValidateCatalog(DefaultCatalog) != nil || !validActorCredential(actorKind, credential) {
		return nil, ErrInvalidSubject
	}
	if actorKind != ActorHuman && role != RoleOperator {
		return nil, ErrRoleForbiddenForActor
	}
	values, exists := DefaultCatalog.Roles[role]
	if !exists {
		return nil, ErrUnknownRole
	}
	result := append([]Capability(nil), values...)
	if credential == AgentToken {
		forbidden := capabilitySet(IntrinsicAgentForbidden)
		for _, capability := range result {
			if forbidden[capability] {
				return nil, ErrRoleForbiddenForActor
			}
		}
	}
	return result, nil
}

func SortedCapabilities(values []Capability) []Capability {
	result := append([]Capability(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

type authzError string

func (e authzError) Error() string { return string(e) }

const (
	ErrInvalidCatalog        authzError = "invalid capability catalog"
	ErrInvalidSubject        authzError = "invalid authorization subject"
	ErrUnknownRole           authzError = "unknown role"
	ErrRoleForbiddenForActor authzError = "role forbidden for actor kind"
)

func knownRole(role Role) bool {
	for _, candidate := range Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func validActorCredential(kind ActorKind, credential CredentialKind) bool {
	return kind == ActorHuman && credential == HumanSession || kind == ActorAgent && credential == AgentToken || kind == ActorSystem && credential == SystemCredential
}

func capabilitySet(values []Capability) map[Capability]bool {
	result := make(map[Capability]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func sameCapabilitySet(left, right []Capability) bool {
	if len(left) != len(right) {
		return false
	}
	leftSet, rightSet := capabilitySet(left), capabilitySet(right)
	if len(leftSet) != len(rightSet) {
		return false
	}
	for capability := range leftSet {
		if !rightSet[capability] {
			return false
		}
	}
	return true
}
