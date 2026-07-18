package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDomainDiagnosticCodesExistInLiteralContract(t *testing.T) {
	path := filepath.Join("..", "..", "..", "contracts", "v1", "diagnostics.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read diagnostics contract: %v", err)
	}
	var contract struct {
		Errors     []string `json:"errors"`
		Warnings   []string `json:"warnings"`
		Advisories []string `json:"advisories"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("parse diagnostics contract: %v", err)
	}
	allCodes := asSet(append(append(contract.Errors, contract.Warnings...), contract.Advisories...))
	for _, code := range UsedDiagnosticCodes {
		if !allCodes[code] {
			t.Errorf("domain diagnostic code %q is missing from diagnostics contract", code)
		}
	}
}

func TestTaskStatusesAndRunKindsExistInStateContract(t *testing.T) {
	path := filepath.Join("..", "..", "..", "contracts", "v1", "states.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read states contract: %v", err)
	}
	var contract struct {
		Workspace struct {
			Values []string `json:"values"`
		} `json:"workspace"`
		Task struct {
			Values []string `json:"values"`
		} `json:"task"`
		Run struct {
			Values   []string `json:"values"`
			Terminal []string `json:"terminal"`
			Kinds    []string `json:"kinds"`
		} `json:"run"`
		TaskRecord struct {
			Values []string `json:"values"`
			Types  []string `json:"types"`
		} `json:"taskRecord"`
		CommitReference struct {
			Relations          []string `json:"relations"`
			VerificationStates []string `json:"verificationStates"`
		} `json:"commitReference"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("parse states contract: %v", err)
	}
	taskValues := asSet(contract.Task.Values)
	for _, status := range TaskStatuses {
		if !taskValues[string(status)] {
			t.Errorf("task status %q is missing from states contract", status)
		}
	}
	runKinds := asSet(contract.Run.Kinds)
	for _, kind := range RunKinds {
		if !runKinds[string(kind)] {
			t.Errorf("run kind %q is missing from states contract", kind)
		}
	}
	runValues := asSet(contract.Run.Values)
	for _, status := range RunStatuses {
		if !runValues[string(status)] {
			t.Errorf("run status %q is missing from states contract", status)
		}
	}
	assertExactSet(t, "workspace state", contract.Workspace.Values, workspaceStateStrings())
	assertExactSet(t, "run status", contract.Run.Values, runStatusStrings())
	assertExactSet(t, "run kind", contract.Run.Kinds, runKindStrings())
	assertExactSet(t, "run terminal status", contract.Run.Terminal, runTerminalStatusStrings())
	assertExactSet(t, "Task Record state", contract.TaskRecord.Values, recordStateStrings())
	assertExactSet(t, "Task Record type", contract.TaskRecord.Types, recordTypeStrings())
	assertExactSet(t, "commit relation", contract.CommitReference.Relations, commitRelationStrings())
	assertExactSet(t, "commit verification state", contract.CommitReference.VerificationStates, commitVerificationStateStrings())
}

func assertExactSet(t *testing.T, label string, contractValues, domainValues []string) {
	t.Helper()
	contractSet := asSet(contractValues)
	domainSet := asSet(domainValues)
	if len(contractValues) != len(contractSet) {
		t.Errorf("%s contract contains duplicate values: %v", label, contractValues)
	}
	if len(domainValues) != len(domainSet) {
		t.Errorf("%s domain contains duplicate values: %v", label, domainValues)
	}
	if len(contractSet) != len(domainSet) {
		t.Errorf("%s contract/domain size mismatch: contract=%v domain=%v", label, contractValues, domainValues)
	}
	for value := range contractSet {
		if !domainSet[value] {
			t.Errorf("%s %q exists in contract but not domain", label, value)
		}
	}
}

func workspaceStateStrings() []string {
	values := make([]string, len(WorkspaceStates))
	for index, value := range WorkspaceStates {
		values[index] = string(value)
	}
	return values
}

func runStatusStrings() []string {
	values := make([]string, len(RunStatuses))
	for index, value := range RunStatuses {
		values[index] = string(value)
	}
	return values
}

func runTerminalStatusStrings() []string {
	values := make([]string, len(RunTerminalStatuses))
	for index, value := range RunTerminalStatuses {
		values[index] = string(value)
	}
	return values
}

func runKindStrings() []string {
	values := make([]string, len(RunKinds))
	for index, value := range RunKinds {
		values[index] = string(value)
	}
	return values
}

func recordStateStrings() []string {
	values := make([]string, len(RecordStates))
	for index, value := range RecordStates {
		values[index] = string(value)
	}
	return values
}
func recordTypeStrings() []string {
	values := make([]string, len(RecordTypes))
	for index, value := range RecordTypes {
		values[index] = string(value)
	}
	return values
}
func commitRelationStrings() []string {
	values := make([]string, len(CommitRelations))
	for index, value := range CommitRelations {
		values[index] = string(value)
	}
	return values
}
func commitVerificationStateStrings() []string {
	values := make([]string, len(CommitVerificationStates))
	for index, value := range CommitVerificationStates {
		values[index] = string(value)
	}
	return values
}

func asSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
