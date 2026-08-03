#!/usr/bin/env python3
"""Validate an append-only Baley PilotMeasurement Markdown Record."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import sys
from pathlib import Path

UUID_RE = re.compile(
    r"^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"
)
SECRET_RE = re.compile(
    r"(?i)(bearer\s+[a-z0-9._~-]{12,}|password\s*[:=]|"
    r"(?:baley[_-]?)?(?:agent[_-]?token|lease[_-]?token(?:[_-]?secret)?)\s*[:=]|"
    r"https?://[^/\s:@]+:[^/\s@]+@)"
)
UTC_RFC3339_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$")
REQUIRED = {
    "measurement_id",
    "workspace_id",
    "lane_id",
    "session_id",
    "sample_id",
    "started_at",
    "ended_at",
    "workspace_revision",
    "actor_id",
    "candidate_ids",
    "accepted_candidate_ids",
    "rejection_reasons",
    "evidence_reference_ids",
    "mismatch_keys",
    "correction_event_ids",
    "gate_id",
    "conversation_ref",
    "human_decision_turn_count",
    "baseline_or_treatment",
}
FRONTMATTER_REQUIRED = {
    "baley_record",
    "record_id",
    "task_id",
    "record_type",
    "run_id",
    "created_at",
    "created_by",
    "supersedes",
}


def parse_time(value: object, field: str, errors: list[str]) -> dt.datetime | None:
    if not isinstance(value, str) or not UTC_RFC3339_RE.fullmatch(value):
        errors.append(f"{field}: expected UTC RFC3339 timestamp ending in Z")
        return None
    try:
        parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        errors.append(f"{field}: invalid RFC3339 timestamp")
        return None
    return parsed


def validate_unique_strings(
    payload: dict[str, object], field: str, errors: list[str]
) -> set[str]:
    value = payload.get(field)
    if not isinstance(value, list) or any(not isinstance(item, str) or not item for item in value):
        errors.append(f"{field}: expected an array of non-empty strings")
        return set()
    if len(value) != len(set(value)):
        errors.append(f"{field}: duplicate values are not allowed")
    return set(value)


def reject_duplicate_keys(pairs: list[tuple[str, object]]) -> dict[str, object]:
    result: dict[str, object] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def extract_frontmatter(text: str) -> dict[str, str]:
    match = re.match(r"^---\n(.*?)\n---(?:\n|$)", text, re.DOTALL)
    if not match:
        raise ValueError("Task Record front matter not found")
    result: dict[str, str] = {}
    for line in match.group(1).splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if ":" not in line:
            raise ValueError(f"invalid front matter line: {line}")
        key, value = line.split(":", 1)
        key, value = key.strip(), value.strip().strip("\"'")
        if key in result:
            raise ValueError(f"duplicate front matter key: {key}")
        result[key] = value
    return result


def extract_payload(text: str) -> dict[str, object]:
    match = re.search(r"```json\s*(\{.*?\})\s*```", text, re.DOTALL)
    if not match:
        raise ValueError("JSON payload fence not found")
    value = json.loads(match.group(1), object_pairs_hook=reject_duplicate_keys)
    if not isinstance(value, dict):
        raise ValueError("measurement payload must be a JSON object")
    return value


def validate(text: str) -> list[str]:
    errors: list[str] = []
    if SECRET_RE.search(text):
        errors.append("content: possible secret or credential-bearing URL")
    try:
        frontmatter = extract_frontmatter(text)
        payload = extract_payload(text)
    except (ValueError, json.JSONDecodeError) as error:
        return sorted(errors + [f"payload: {error}"])

    if frontmatter.get("baley_record") != "1":
        errors.append("frontmatter.baley_record: expected 1")
    missing_frontmatter = FRONTMATTER_REQUIRED - frontmatter.keys()
    unknown_frontmatter = frontmatter.keys() - FRONTMATTER_REQUIRED
    errors.extend(
        f"frontmatter.{field}: required field is missing"
        for field in missing_frontmatter
    )
    errors.extend(
        f"frontmatter.{field}: unknown field" for field in unknown_frontmatter
    )
    if frontmatter.get("record_type") != "pilot-measurement":
        errors.append("frontmatter.record_type: expected pilot-measurement")
    record_id = frontmatter.get("record_id", "")
    if not UUID_RE.fullmatch(record_id):
        errors.append("frontmatter.record_id: expected lowercase UUID")
    task_id = frontmatter.get("task_id", "")
    if not task_id.isdigit() or int(task_id) < 1:
        errors.append("frontmatter.task_id: expected positive integer")
    if not UUID_RE.fullmatch(frontmatter.get("run_id", "")):
        errors.append("frontmatter.run_id: expected lowercase UUID")
    parse_time(frontmatter.get("created_at"), "frontmatter.created_at", errors)
    if not frontmatter.get("created_by", "").strip():
        errors.append("frontmatter.created_by: expected non-empty string")
    supersedes = frontmatter.get("supersedes", "")
    if supersedes != "null" and not UUID_RE.fullmatch(supersedes):
        errors.append("frontmatter.supersedes: expected null or lowercase UUID")

    missing = REQUIRED - payload.keys()
    unknown = payload.keys() - REQUIRED
    errors.extend(f"{field}: required field is missing" for field in missing)
    errors.extend(f"{field}: unknown field" for field in unknown)

    for field in ("measurement_id", "workspace_id"):
        value = payload.get(field)
        if not isinstance(value, str) or not UUID_RE.fullmatch(value):
            errors.append(f"{field}: expected lowercase UUID")
    if record_id and payload.get("measurement_id") != record_id:
        errors.append("measurement_id: must equal frontmatter record_id")
    for field in ("lane_id", "session_id", "sample_id", "actor_id", "gate_id", "conversation_ref"):
        if not isinstance(payload.get(field), str) or not str(payload.get(field)).strip():
            errors.append(f"{field}: expected non-empty string")

    started = parse_time(payload.get("started_at"), "started_at", errors)
    ended = parse_time(payload.get("ended_at"), "ended_at", errors)
    if started and ended and ended < started:
        errors.append("ended_at: must not be earlier than started_at")

    revision = payload.get("workspace_revision")
    if not isinstance(revision, int) or isinstance(revision, bool) or revision < 1:
        errors.append("workspace_revision: expected positive integer")
    turns = payload.get("human_decision_turn_count")
    if not isinstance(turns, int) or isinstance(turns, bool) or turns < 0:
        errors.append("human_decision_turn_count: expected non-negative integer")
    if payload.get("baseline_or_treatment") not in ("baseline", "treatment"):
        errors.append("baseline_or_treatment: expected baseline or treatment")

    candidates = validate_unique_strings(payload, "candidate_ids", errors)
    accepted = validate_unique_strings(payload, "accepted_candidate_ids", errors)
    if any(":" in candidate for candidate in candidates):
        errors.append("candidate_ids: colon is reserved for rejection reason separation")
    if not accepted.issubset(candidates):
        errors.append("accepted_candidate_ids: every value must appear in candidate_ids")
    reasons = validate_unique_strings(payload, "rejection_reasons", errors)
    rejected = candidates - accepted
    reason_candidates: list[str] = []
    for reason in reasons:
        if ":" in reason and reason.split(":", 1)[1].strip():
            reason_candidates.append(reason.split(":", 1)[0])
    if set(reason_candidates) != rejected or len(reason_candidates) != len(rejected):
        errors.append("rejection_reasons: require one candidate-id:reason entry for every non-accepted candidate")
    for field in ("evidence_reference_ids", "mismatch_keys", "correction_event_ids"):
        validate_unique_strings(payload, field, errors)
    return sorted(set(errors))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("measurement", type=Path)
    args = parser.parse_args()
    try:
        text = args.measurement.read_text(encoding="utf-8")
    except OSError as error:
        print(f"ERROR file: {error}")
        return 2
    errors = validate(text)
    if errors:
        for error in errors:
            print(f"ERROR {error}")
        return 1
    print("PASS pilot-measurement")
    return 0


if __name__ == "__main__":
    sys.exit(main())
