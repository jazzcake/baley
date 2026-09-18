-- +goose Up
ALTER TABLE conversational_decision_evidence
  DROP CONSTRAINT conversational_decision_evidence_action_check;
ALTER TABLE conversational_decision_evidence
  ADD CONSTRAINT conversational_decision_evidence_action_check
  CHECK (action IN ('task.confirm','task.discard'));

-- +goose Down
ALTER TABLE conversational_decision_evidence
  DROP CONSTRAINT conversational_decision_evidence_action_check;
ALTER TABLE conversational_decision_evidence
  ADD CONSTRAINT conversational_decision_evidence_action_check
  CHECK (action='task.confirm');
