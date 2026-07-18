package authz

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCapabilityCatalogMatchesLiteralContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "v1", "capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	var literal struct {
		Capabilities []Capability          `json:"capabilities"`
		Roles        map[Role][]Capability `json:"roles"`
		AgentToken   struct {
			AllowedRole           Role         `json:"allowedRole"`
			ForbiddenCapabilities []Capability `json:"forbiddenCapabilities"`
		} `json:"agentToken"`
	}
	if err := json.Unmarshal(data, &literal); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(literal.Capabilities, DefaultCatalog.Capabilities) || !reflect.DeepEqual(literal.Roles, DefaultCatalog.Roles) || literal.AgentToken.AllowedRole != DefaultCatalog.AgentAllowedRole || !reflect.DeepEqual(literal.AgentToken.ForbiddenCapabilities, DefaultCatalog.AgentForbidden) {
		t.Fatalf("capability literal drift: %+v vs %+v", literal, DefaultCatalog)
	}
	if err := ValidateCatalog(DefaultCatalog); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRoleCoversEveryRoleCapabilityAndActorBoundary(t *testing.T) {
	for _, role := range Roles {
		values, err := ResolveRole(role, ActorHuman, HumanSession)
		if err != nil || !reflect.DeepEqual(values, DefaultCatalog.Roles[role]) {
			t.Errorf("human role %s: %v %v", role, values, err)
		}
	}
	operator, err := ResolveRole(RoleOperator, ActorAgent, AgentToken)
	if err != nil || !reflect.DeepEqual(operator, DefaultCatalog.Roles[RoleOperator]) {
		t.Fatalf("Agent operator rejected: %v %v", operator, err)
	}
	for _, role := range []Role{RoleViewer, RoleApprover, RoleOwner} {
		if _, err := ResolveRole(role, ActorAgent, AgentToken); err != ErrRoleForbiddenForActor {
			t.Errorf("Agent role %s accepted: %v", role, err)
		}
	}
	if _, err := ResolveRole(RoleOperator, ActorHuman, AgentToken); err != ErrInvalidSubject {
		t.Fatalf("mismatched Actor credential accepted: %v", err)
	}
	if values, err := ResolveRole(RoleOperator, ActorSystem, SystemCredential); err != nil || !reflect.DeepEqual(values, DefaultCatalog.Roles[RoleOperator]) {
		t.Fatalf("system operator rejected: %v %v", values, err)
	}
	if _, err := ResolveRole(RoleOwner, ActorSystem, SystemCredential); err != ErrRoleForbiddenForActor {
		t.Fatalf("system gained owner role: %v", err)
	}
}

func TestValidateCatalogRejectsUnknownDuplicateAndAgentEscalation(t *testing.T) {
	duplicate := DefaultCatalog
	duplicate.Capabilities = append(append([]Capability(nil), Capabilities...), WorkspaceRead)
	if ValidateCatalog(duplicate) == nil {
		t.Fatal("duplicate capability accepted")
	}
	unknown := cloneCatalog(DefaultCatalog)
	unknown.Roles[RoleViewer] = []Capability{"unknown"}
	if ValidateCatalog(unknown) == nil {
		t.Fatal("unknown role capability accepted")
	}
	escalated := cloneCatalog(DefaultCatalog)
	escalated.Roles[RoleOperator] = append(escalated.Roles[RoleOperator], GateApprove)
	if ValidateCatalog(escalated) == nil {
		t.Fatal("Agent allowed role gained forbidden capability")
	}
	combined := cloneCatalog(DefaultCatalog)
	combined.AgentForbidden = []Capability{TaskApprove, LaneApprove, WorkspaceClose, WorkspaceAdmin}
	combined.Roles[RoleOperator] = append(combined.Roles[RoleOperator], GateApprove)
	if ValidateCatalog(combined) == nil {
		t.Fatal("Agent escalation accepted after forbidden-list removal")
	}
	duplicateForbidden := cloneCatalog(DefaultCatalog)
	duplicateForbidden.AgentForbidden = append(duplicateForbidden.AgentForbidden, GateApprove)
	if ValidateCatalog(duplicateForbidden) == nil {
		t.Fatal("duplicate Agent forbidden capability accepted")
	}
}

func cloneCatalog(value Catalog) Catalog {
	result := value
	result.Capabilities = append([]Capability(nil), value.Capabilities...)
	result.AgentForbidden = append([]Capability(nil), value.AgentForbidden...)
	result.Roles = map[Role][]Capability{}
	for role, capabilities := range value.Roles {
		result.Roles[role] = append([]Capability(nil), capabilities...)
	}
	return result
}
