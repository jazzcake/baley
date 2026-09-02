package postgres

import (
	"reflect"
	"testing"

	"github.com/jazzcake/baley/server/internal/authz"
)

func TestAgentScopesForMemberRole(t *testing.T) {
	tests := []struct {
		name string
		role authz.Role
		want []authz.Capability
	}{
		{name: "viewer", role: authz.RoleViewer, want: []authz.Capability{authz.WorkspaceRead}},
		{name: "approver", role: authz.RoleApprover, want: []authz.Capability{authz.WorkspaceRead}},
		{name: "operator", role: authz.RoleOperator, want: []authz.Capability{authz.WorkspaceRead, authz.WorkspaceOperate, authz.RunOperate, authz.RecordOperate}},
		{name: "owner", role: authz.RoleOwner, want: []authz.Capability{authz.WorkspaceRead, authz.WorkspaceOperate, authz.RunOperate, authz.RecordOperate}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := agentScopesForMemberRole(test.role)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("agentScopesForMemberRole(%q) = %v, want %v", test.role, got, test.want)
			}
			for _, capability := range got {
				if capability == authz.TaskApprove || capability == authz.LaneApprove || capability == authz.GateApprove {
					t.Fatalf("human-only capability %q leaked to Agent scope", capability)
				}
			}
		})
	}
}
