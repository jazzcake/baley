-- +goose Up
ALTER TABLE task_record_indexes
  DROP CONSTRAINT task_record_indexes_record_type_check;

ALTER TABLE task_record_indexes
  ADD CONSTRAINT task_record_indexes_record_type_check
  CHECK (record_type IN (
    'detailed-plan',
    'handoff',
    'independent-agent-review',
    'review-response',
    'completion-report',
    'pilot-measurement'
  ));

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM task_record_indexes WHERE record_type='pilot-measurement'
  ) THEN
    RAISE EXCEPTION
      'cannot downgrade while pilot-measurement Task Records exist';
  END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE task_record_indexes
  DROP CONSTRAINT task_record_indexes_record_type_check;

ALTER TABLE task_record_indexes
  ADD CONSTRAINT task_record_indexes_record_type_check
  CHECK (record_type IN (
    'detailed-plan',
    'handoff',
    'independent-agent-review',
    'review-response',
    'completion-report'
  ));
