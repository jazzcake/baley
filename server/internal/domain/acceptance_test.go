package domain

import "testing"

func TestResolveAcceptanceUsesImmutablePolicySnapshot(t *testing.T) {
	policy := AcceptancePolicy{
		WorkspaceID: "workspace", PolicyVersion: "pilot-v2",
		DefaultMode: AcceptanceHumanRequired, EvidenceProfileID: "technical-v1",
	}
	assignment, err := ResolveAcceptance(policy, AcceptanceInherit, "")
	if err != nil || assignment.EffectiveMode != AcceptanceHumanRequired ||
		assignment.PolicyVersion != "pilot-v2" || assignment.EvidenceProfileID != "technical-v1" {
		t.Fatalf("inherit resolution failed: %+v %v", assignment, err)
	}
	policy.PolicyVersion = "pilot-v3"
	if assignment.PolicyVersion != "pilot-v2" || assignment.EffectiveMode != AcceptanceHumanRequired {
		t.Fatalf("existing assignment changed with future policy: %+v", assignment)
	}
	if _, err = ResolveAcceptance(policy, AcceptanceDelegated, ""); acceptanceViolationCode(err) != CodeInvalidStateTransition {
		t.Fatalf("delegated request was not rejected: %v", err)
	}
	if _, err = ResolveAcceptance(policy, AcceptanceHumanRequired, "weaker-v1"); acceptanceViolationCode(err) != CodeHumanApprovalRequired {
		t.Fatalf("ad-hoc profile weakening accepted: %v", err)
	}
}

func TestAcceptanceEvaluationAndEscalationAreConservative(t *testing.T) {
	profile := EvidenceProfile{
		AllowedReferenceKinds:         []string{"artifact"},
		VerificationReferenceRequired: true, ReviewRequiresZeroBlockers: true,
	}
	evidence := TaskAcceptanceEvidence{
		CompletionReportRecordID: "completion", VerificationVerdict: EvidencePassed,
		VerificationReference: "go test ./...", VerificationReferenceKind: "artifact",
		IndependentReviewRecordID: "review", ReviewVerdict: ReviewPass,
	}
	if evaluation := EvaluateAcceptance(profile, evidence); !evaluation.Eligible || len(evaluation.Reasons) != 0 {
		t.Fatalf("eligible evidence rejected: %+v", evaluation)
	}
	evidence.UnresolvedBlockingCount = 1
	if evaluation := EvaluateAcceptance(profile, evidence); evaluation.Eligible || !containsString(evaluation.Reasons, "unresolved_blocking_findings") {
		t.Fatalf("blocking review accepted: %+v", evaluation)
	}

	current := TaskAcceptanceAssignment{
		ID: "assignment-v1", WorkspaceID: "workspace", TaskID: "task", Version: 1,
		RequestedMode: AcceptanceDelegated, EffectiveMode: AcceptanceDelegated,
		PolicyVersion: "pilot-v2", EvidenceProfileID: "technical-v1",
	}
	next, err := EscalateAcceptance(
		Task{ID: "task", WorkspaceID: "workspace", Status: TaskImplemented},
		current, "assignment-v2", "risk increased", "review:42", "pilot-v2", "owner",
	)
	if err != nil || next.EffectiveMode != AcceptanceHumanRequired ||
		next.SupersedesAssignmentID != current.ID || next.Version != 2 {
		t.Fatalf("escalation failed: %+v %v", next, err)
	}
	if _, err = EscalateAcceptance(
		Task{ID: "task", WorkspaceID: "workspace", Status: TaskImplemented},
		next, "assignment-v3", "de-escalate", "review:43", "pilot-v2", "owner",
	); acceptanceViolationCode(err) != CodeInvalidStateTransition {
		t.Fatalf("de-escalation path was accepted: %v", err)
	}
}

func acceptanceViolationCode(err error) string {
	if violation, ok := err.(*Violation); ok {
		return violation.Code
	}
	return ""
}
