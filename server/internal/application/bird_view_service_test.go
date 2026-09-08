package application

import (
	"testing"

	"github.com/jazzcake/baley/server/internal/authz"
)

func TestBirdViewHashesIgnoreCredentialWorkspaceRouting(t *testing.T) {
	left := birdViewArgs{CredentialWorkspaceID: "workspace-a", BirdViewID: "20000000-0000-4000-8000-000000000026"}
	right := left
	right.CredentialWorkspaceID = "workspace-b"
	if birdViewCommandHash("bird_view.archive", "account", 4, left) != birdViewCommandHash("bird_view.archive", "account", 4, right) {
		t.Fatal("account-scoped command hash changed with credential Workspace")
	}
	request := CommandRequest{Name: "bird_view.archive", Principal: &CommandPrincipal{AccountID: "account", CredentialID: "credential", Subject: authz.Subject{ActorID: "actor"}}, Envelope: CommandEnvelope{ExpectedBirdViewRevision: 4, ExecutedByActorID: "actor"}}
	if birdViewFingerprint(request, left) != birdViewFingerprint(request, right) {
		t.Fatal("account-scoped idempotency fingerprint changed with credential Workspace")
	}
}
