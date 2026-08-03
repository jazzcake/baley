package httpapi

import (
	"errors"
	"testing"
)

func TestMCPConnectionBrokerRequiresSecretAndConsumesApprovedToken(t *testing.T) {
	broker := NewMCPConnectionBroker()
	view, secret, err := broker.Create("410f335e-ddb2-443f-be3c-7d1d18ccd534", "agent")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = broker.Poll(view.ID, "wrong"); !errors.Is(err, errMCPConnectionSecret) {
		t.Fatalf("wrong secret was accepted: %v", err)
	}
	if _, err = broker.Approve(view.ID, view.WorkspaceID, "operator-token"); err != nil {
		t.Fatal(err)
	}
	approved, token, err := broker.Poll(view.ID, secret)
	if err != nil || approved.Status != "approved" || token != "operator-token" {
		t.Fatalf("approved token not delivered: view=%#v token=%q err=%v", approved, token, err)
	}
	if _, _, err = broker.Poll(view.ID, secret); !errors.Is(err, errMCPConnectionNotFound) {
		t.Fatalf("token should only be delivered once: %v", err)
	}
}
