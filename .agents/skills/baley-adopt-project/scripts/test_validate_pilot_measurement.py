import importlib.util
import json
import pathlib
import sys
import unittest

sys.dont_write_bytecode = True
SCRIPT = pathlib.Path(__file__).with_name("validate_pilot_measurement.py")
SPEC = importlib.util.spec_from_file_location("validator", SCRIPT)
validator = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(validator)


def markdown(payload):
    return (
        "---\n"
        "baley_record: 1\n"
        'record_id: "6279cb62-d52f-4642-942c-15e7bd72c9bf"\n'
        "task_id: 124\n"
        "record_type: pilot-measurement\n"
        'run_id: "9f74c155-6e82-4ea3-bb0f-2c85abe01416"\n'
        'created_at: "2026-07-30T14:05:00Z"\n'
        'created_by: "operator"\n'
        "supersedes: null\n"
        "---\n\n"
        "```json\n" + json.dumps(payload) + "\n```\n"
    )


def valid_payload():
    return {
        "measurement_id": "6279cb62-d52f-4642-942c-15e7bd72c9bf",
        "workspace_id": "00000000-0000-4000-8000-000000000001",
        "lane_id": "adoption",
        "session_id": "session-01",
        "sample_id": "sample-01",
        "started_at": "2026-07-30T14:00:00Z",
        "ended_at": "2026-07-30T14:05:00Z",
        "workspace_revision": 472,
        "actor_id": "operator",
        "candidate_ids": ["candidate-1", "candidate-2"],
        "accepted_candidate_ids": ["candidate-1"],
        "rejection_reasons": ["candidate-2:not-applicable"],
        "evidence_reference_ids": ["record:one"],
        "mismatch_keys": ["record:one:working-tree"],
        "correction_event_ids": [],
        "gate_id": "G#4",
        "conversation_ref": "codex:test",
        "human_decision_turn_count": 0,
        "baseline_or_treatment": "treatment",
    }


class ValidatorTest(unittest.TestCase):
    def test_valid(self):
        self.assertEqual(validator.validate(markdown(valid_payload())), [])

    def test_reports_cross_field_and_secret_errors_stably(self):
        payload = valid_payload()
        payload["accepted_candidate_ids"] = ["missing"]
        payload["ended_at"] = "2026-07-30T13:00:00Z"
        text = markdown(payload) + "\npassword=do-not-store"
        errors = validator.validate(text)
        self.assertEqual(errors, sorted(errors))
        self.assertIn(
            "accepted_candidate_ids: every value must appear in candidate_ids", errors
        )
        self.assertIn("ended_at: must not be earlier than started_at", errors)
        self.assertIn("content: possible secret or credential-bearing URL", errors)

    def test_rejects_duplicates_unknown_and_missing(self):
        payload = valid_payload()
        del payload["gate_id"]
        payload["candidate_ids"] = ["same", "same"]
        payload["extra"] = True
        errors = validator.validate(markdown(payload))
        self.assertIn("gate_id: required field is missing", errors)
        self.assertIn("candidate_ids: duplicate values are not allowed", errors)
        self.assertIn("extra: unknown field", errors)

    def test_rejects_operational_secrets_non_utc_and_missing_record_header(self):
        payload = valid_payload()
        payload["started_at"] = "2026-07-30 14:00:00+09:00"
        text = markdown(payload) + "\nBALEY_AGENT_TOKEN=abcdefghijklmnop"
        errors = validator.validate(text)
        self.assertIn("content: possible secret or credential-bearing URL", errors)
        self.assertIn("started_at: expected UTC RFC3339 timestamp ending in Z", errors)
        self.assertTrue(validator.validate("```json\n{}\n```"))

    def test_rejects_duplicate_json_keys_and_record_identity_mismatch(self):
        payload = valid_payload()
        payload["measurement_id"] = "6279cb62-d52f-4642-942c-15e7bd72c9be"
        self.assertIn(
            "measurement_id: must equal frontmatter record_id",
            validator.validate(markdown(payload)),
        )
        duplicate = markdown(valid_payload()).replace(
            '"lane_id": "adoption"', '"lane_id": "adoption", "lane_id": "other"'
        )
        self.assertTrue(any("duplicate JSON key" in error for error in validator.validate(duplicate)))

    def test_requires_complete_record_header_and_one_reason_per_rejected_candidate(self):
        payload = valid_payload()
        payload["rejection_reasons"] = [
            "candidate-2:not-applicable",
            "candidate-2:duplicate-explanation",
        ]
        self.assertIn(
            "rejection_reasons: require one candidate-id:reason entry for every non-accepted candidate",
            validator.validate(markdown(payload)),
        )
        missing_created_by = markdown(valid_payload()).replace(
            'created_by: "operator"\n', ""
        )
        self.assertIn(
            "frontmatter.created_by: required field is missing",
            validator.validate(missing_created_by),
        )
        payload = valid_payload()
        payload["candidate_ids"] = ["candidate:ambiguous"]
        payload["accepted_candidate_ids"] = []
        payload["rejection_reasons"] = ["candidate:ambiguous:reason"]
        self.assertIn(
            "candidate_ids: colon is reserved for rejection reason separation",
            validator.validate(markdown(payload)),
        )


if __name__ == "__main__":
    unittest.main()
