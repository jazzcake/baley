package domain

import "strings"

type AcceptanceMode string

const (
	AcceptanceDelegated     AcceptanceMode = "delegated"
	AcceptanceHumanRequired AcceptanceMode = "human_required"
	AcceptanceInherit       AcceptanceMode = "inherit"
)

type EvidenceVerdict string

const (
	EvidencePassed      EvidenceVerdict = "passed"
	EvidenceFailed      EvidenceVerdict = "failed"
	EvidenceUnavailable EvidenceVerdict = "unavailable"
)

type ReviewVerdict string

const (
	ReviewPass        ReviewVerdict = "pass"
	ReviewFail        ReviewVerdict = "fail"
	ReviewUnavailable ReviewVerdict = "unavailable"
)

type AcceptancePolicy struct {
	WorkspaceID       string         `json:"workspaceId"`
	PolicyVersion     string         `json:"policyVersion"`
	DefaultMode       AcceptanceMode `json:"defaultMode"`
	EvidenceProfileID string         `json:"evidenceProfileId"`
}

type EvidenceProfile struct {
	ID                            string   `json:"id"`
	WorkspaceID                   string   `json:"workspaceId"`
	Version                       string   `json:"version"`
	AllowedReferenceKinds         []string `json:"allowedReferenceKinds"`
	VerificationReferenceRequired bool     `json:"verificationReferenceRequired"`
	ReviewRequiresZeroBlockers    bool     `json:"reviewRequiresZeroBlockers"`
}

type TaskAcceptanceAssignment struct {
	ID                     string         `json:"id"`
	WorkspaceID            string         `json:"workspaceId"`
	TaskID                 string         `json:"taskId"`
	Version                int            `json:"version"`
	RequestedMode          AcceptanceMode `json:"requestedMode"`
	EffectiveMode          AcceptanceMode `json:"effectiveMode"`
	PolicyVersion          string         `json:"policyVersion"`
	EvidenceProfileID      string         `json:"evidenceProfileId"`
	Reason                 string         `json:"reason,omitempty"`
	EvidenceReference      string         `json:"evidenceReference,omitempty"`
	ApprovedByActorID      string         `json:"approvedByActorId,omitempty"`
	SupersedesAssignmentID string         `json:"supersedesAssignmentId,omitempty"`
}

type TaskAcceptanceEvidence struct {
	ID                        string          `json:"id"`
	WorkspaceID               string          `json:"workspaceId"`
	TaskID                    string          `json:"taskId"`
	Version                   int             `json:"version"`
	CompletionReportRecordID  string          `json:"completionReportRecordId"`
	VerificationVerdict       EvidenceVerdict `json:"verificationVerdict"`
	VerificationReference     string          `json:"verificationReference,omitempty"`
	VerificationReferenceKind string          `json:"verificationReferenceKind,omitempty"`
	IndependentReviewRecordID string          `json:"independentReviewRecordId"`
	ReviewVerdict             ReviewVerdict   `json:"reviewVerdict"`
	UnresolvedBlockingCount   int             `json:"unresolvedBlockingCount"`
	CommitReferenceID         string          `json:"commitReferenceId,omitempty"`
	ReportedByActorID         string          `json:"reportedByActorId"`
}

type AcceptanceEvaluation struct {
	Eligible bool     `json:"eligible"`
	Reasons  []string `json:"reasons"`
}

func ResolveAcceptance(policy AcceptancePolicy, requested AcceptanceMode, profileID string) (TaskAcceptanceAssignment, error) {
	if policy.WorkspaceID == "" || strings.TrimSpace(policy.PolicyVersion) == "" || (policy.DefaultMode != AcceptanceDelegated && policy.DefaultMode != AcceptanceHumanRequired) || strings.TrimSpace(policy.EvidenceProfileID) == "" {
		return TaskAcceptanceAssignment{}, &Violation{Code: CodeInvalidStateTransition}
	}
	if requested == "" {
		requested = AcceptanceInherit
	}
	effective := requested
	if requested == AcceptanceInherit {
		effective = policy.DefaultMode
	}
	if effective != AcceptanceDelegated && effective != AcceptanceHumanRequired {
		return TaskAcceptanceAssignment{}, &Violation{Code: CodeInvalidStateTransition}
	}
	if requested == AcceptanceDelegated && policy.DefaultMode != AcceptanceDelegated {
		return TaskAcceptanceAssignment{}, &Violation{Code: CodeHumanApprovalRequired}
	}
	if strings.TrimSpace(profileID) == "" {
		profileID = policy.EvidenceProfileID
	}
	if strings.TrimSpace(profileID) != policy.EvidenceProfileID {
		return TaskAcceptanceAssignment{}, &Violation{Code: CodeHumanApprovalRequired}
	}
	return TaskAcceptanceAssignment{
		RequestedMode: requested, EffectiveMode: effective, PolicyVersion: policy.PolicyVersion,
		EvidenceProfileID: strings.TrimSpace(profileID), Version: 1,
	}, nil
}

func EvaluateAcceptance(profile EvidenceProfile, evidence TaskAcceptanceEvidence) AcceptanceEvaluation {
	result := AcceptanceEvaluation{Eligible: true, Reasons: []string{}}
	if strings.TrimSpace(evidence.CompletionReportRecordID) == "" {
		result.Reasons = append(result.Reasons, "completion_report_missing")
	}
	if evidence.VerificationVerdict != EvidencePassed {
		result.Reasons = append(result.Reasons, "verification_not_passed")
	}
	if profile.VerificationReferenceRequired && strings.TrimSpace(evidence.VerificationReference) == "" {
		result.Reasons = append(result.Reasons, "verification_reference_missing")
	}
	if !containsString(profile.AllowedReferenceKinds, evidence.VerificationReferenceKind) {
		result.Reasons = append(result.Reasons, "verification_reference_kind_not_allowed")
	}
	if strings.TrimSpace(evidence.IndependentReviewRecordID) == "" {
		result.Reasons = append(result.Reasons, "independent_review_missing")
	}
	if evidence.ReviewVerdict != ReviewPass {
		result.Reasons = append(result.Reasons, "review_not_passed")
	}
	if profile.ReviewRequiresZeroBlockers && evidence.UnresolvedBlockingCount != 0 {
		result.Reasons = append(result.Reasons, "unresolved_blocking_findings")
	}
	result.Eligible = len(result.Reasons) == 0
	return result
}

func EscalateAcceptance(task Task, current TaskAcceptanceAssignment, nextID, reason, evidenceReference, policyVersion, approvedBy string) (TaskAcceptanceAssignment, error) {
	if current.EffectiveMode != AcceptanceDelegated || (task.Status != TaskPending && task.Status != TaskInProgress && task.Status != TaskImplemented) ||
		strings.TrimSpace(nextID) == "" || strings.TrimSpace(reason) == "" || strings.TrimSpace(evidenceReference) == "" || strings.TrimSpace(policyVersion) == "" || strings.TrimSpace(approvedBy) == "" {
		return TaskAcceptanceAssignment{}, &Violation{Code: CodeInvalidStateTransition}
	}
	next := current
	next.ID, next.Version = nextID, current.Version+1
	next.EffectiveMode = AcceptanceHumanRequired
	next.PolicyVersion = strings.TrimSpace(policyVersion)
	next.Reason = strings.TrimSpace(reason)
	next.EvidenceReference = strings.TrimSpace(evidenceReference)
	next.ApprovedByActorID = approvedBy
	next.SupersedesAssignmentID = current.ID
	return next, nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
