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
		Task struct {
			Values []string `json:"values"`
		} `json:"task"`
		Run struct {
			Kinds []string `json:"kinds"`
		} `json:"run"`
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
}

func asSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
