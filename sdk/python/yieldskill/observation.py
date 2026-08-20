"""Strict yield.observation.v1 receipt types, parsing, and verification."""

from __future__ import annotations

import hashlib
import json
import re
from datetime import datetime
from typing import Any, Literal, Mapping, TypedDict, cast


OperationKind = Literal["ask_user", "agent_task", "run_command"]
LifecyclePhase = Literal[
    "initializing",
    "initialization_failed",
    "awaiting_response",
    "advancing",
    "recoverable_error",
    "diverged",
    "terminal",
]
TerminalDisposition = Literal["completed", "blocked", "refused"]
TerminalCause = Literal[
    "completed", "blocked", "refused", "requirement_failed", "completion_unproven"
]
FailureCode = Literal[
    "manifest_invalid",
    "manifest_read_failed",
    "runtime_version_missing",
    "runtime_incompatible",
    "runner_missing",
    "source_digest_failed",
    "initialization_failed",
    "invalid_program_output",
    "execution_timeout",
    "subprocess_failed",
    "execution_failed",
    "command_execution_failed",
]
RequirementResult = Literal["passed", "failed"]
ResponseRejectionReason = Literal[
    "wrong-run",
    "stale-response",
    "duplicate-response",
    "wrong-request",
    "schema-invalid",
    "digest-mismatch",
    "completion-unproven",
    "run-closed",
    "no-pending-operation",
]
ExperimentRole = Literal["baseline", "candidate"]


class JournalBinding(TypedDict):
    run_id: str
    head_sequence: int
    head_digest: str


class _RunIdentityRequired(TypedDict):
    id: str


class RunIdentity(_RunIdentityRequired, total=False):
    input_digest: str


class ProfileDigest(TypedDict):
    profile: str
    value: str


class _SkillIdentityRequired(TypedDict):
    name: str


class SkillIdentity(_SkillIdentityRequired, total=False):
    version: str
    binding_digest: str
    source_digest: ProfileDigest


class RuntimeIdentity(TypedDict, total=False):
    supervisor_version: str
    required_version: str
    compatible: bool


class _TimingSummaryRequired(TypedDict):
    started_at: str
    last_observed_at: str


class TimingSummary(_TimingSummaryRequired, total=False):
    ended_at: str
    elapsed_ms: int
    clock_anomaly: bool


class _OperationObservationRequired(TypedDict):
    sequence: int
    kind: OperationKind
    operation_key_digest: str
    requested_at: str


class OperationObservation(_OperationObservationRequired, total=False):
    completed_at: str
    elapsed_ms: int
    result_digest: str
    clock_anomaly: bool


class OperationSummary(TypedDict):
    kind: OperationKind
    requested: int
    completed: int
    total_elapsed_ms: int


class _OutcomeSummaryRequired(TypedDict):
    phase: LifecyclePhase


class OutcomeSummary(_OutcomeSummaryRequired, total=False):
    terminal_disposition: TerminalDisposition
    terminal_cause: TerminalCause
    result_digest: str
    failure_code: FailureCode


class _RequirementOutcomeRequired(TypedDict):
    outcome: RequirementResult
    claim_digest: str


class RequirementOutcome(_RequirementOutcomeRequired, total=False):
    evidence_digest: str


class ResponseRejectionSummary(TypedDict):
    reason: ResponseRejectionReason
    count: int


class DivergenceOutcome(TypedDict):
    sequence: int
    expected_digest: str
    got_digest: str


class _ExperimentContextRequired(TypedDict):
    experiment_id: str
    variant_id: str
    role: ExperimentRole


class ExperimentContext(_ExperimentContextRequired, total=False):
    cohort_id: str
    baseline_variant_id: str
    parent_skill_version: str


class _RunReceiptRequired(TypedDict):
    schema: Literal["yield.observation.v1"]
    kind: Literal["run_receipt"]
    receipt_digest: str
    journal: JournalBinding
    run: RunIdentity
    skill: SkillIdentity
    timing: TimingSummary
    operations: list[OperationObservation]
    operation_summaries: list[OperationSummary]
    outcome: OutcomeSummary
    requirements: list[RequirementOutcome]
    response_rejections: list[ResponseRejectionSummary]
    divergences: list[DivergenceOutcome]


class RunReceipt(_RunReceiptRequired, total=False):
    runtime: RuntimeIdentity
    experiment: ExperimentContext


class ReceiptError(ValueError):
    """The receipt is not strict, canonical, or digest-valid."""


_DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")
_SEMVER = re.compile(r"^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$")
_SOURCE_PROFILE = re.compile(r"^yield\.skill-source\.v[0-9]+$")
_IDENTIFIER = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$")
_TIMESTAMP = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$")

_OPERATION_KINDS = {"ask_user", "agent_task", "run_command"}
_PHASES = {
    "initializing",
    "initialization_failed",
    "awaiting_response",
    "advancing",
    "recoverable_error",
    "diverged",
    "terminal",
}
_DISPOSITIONS = {"completed", "blocked", "refused"}
_TERMINAL_CAUSES = {
    "completed",
    "blocked",
    "refused",
    "requirement_failed",
    "completion_unproven",
}
_FAILURE_CODES = {
    "manifest_invalid",
    "manifest_read_failed",
    "runtime_version_missing",
    "runtime_incompatible",
    "runner_missing",
    "source_digest_failed",
    "initialization_failed",
    "invalid_program_output",
    "execution_timeout",
    "subprocess_failed",
    "execution_failed",
    "command_execution_failed",
}
_REJECTION_REASONS = {
    "wrong-run",
    "stale-response",
    "duplicate-response",
    "wrong-request",
    "schema-invalid",
    "digest-mismatch",
    "completion-unproven",
    "run-closed",
    "no-pending-operation",
}


def _object(value: Any, path: str, allowed: set[str], required: set[str]) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ReceiptError(f"{path}: must be an object")
    unknown = set(value) - allowed
    if unknown:
        raise ReceiptError(f"{path}: unknown field {sorted(unknown)[0]!r}")
    missing = required - set(value)
    if missing:
        raise ReceiptError(f"{path}: missing field {sorted(missing)[0]!r}")
    return value


def _string(value: Any, path: str, pattern: re.Pattern[str] | None = None) -> str:
    if (
        not isinstance(value, str)
        or not value
        or (pattern and not pattern.fullmatch(value))
        or any(0xD800 <= ord(character) <= 0xDFFF for character in value)
    ):
        raise ReceiptError(f"{path}: must be a valid non-empty string")
    return value


def _timestamp(value: Any, path: str) -> str:
    text = _string(value, path, _TIMESTAMP)
    try:
        datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError as error:
        raise ReceiptError(f"{path}: must be an RFC 3339 timestamp") from error
    return text


def _integer(value: Any, path: str, minimum: int) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < minimum:
        raise ReceiptError(f"{path}: must be an integer >= {minimum}")
    return value


def _boolean(value: Any, path: str) -> bool:
    if not isinstance(value, bool):
        raise ReceiptError(f"{path}: must be a boolean")
    return value


def _one_of(value: Any, path: str, values: set[str]) -> str:
    if not isinstance(value, str) or value not in values:
        raise ReceiptError(f"{path}: invalid value")
    return value


def _optional(value: dict[str, Any], key: str, validator: Any, path: str) -> None:
    if key in value:
        validator(value[key], f"{path}.{key}")


def _array(value: dict[str, Any], key: str, validator: Any, path: str = "$") -> None:
    items = value[key]
    if not isinstance(items, list):
        raise ReceiptError(f"{path}.{key}: must be an array")
    for index, item in enumerate(items):
        validator(item, f"{path}.{key}[{index}]")


def _validate_receipt(value: Any) -> None:
    receipt = _object(
        value,
        "$",
        {
            "schema",
            "kind",
            "receipt_digest",
            "journal",
            "run",
            "skill",
            "runtime",
            "timing",
            "operations",
            "operation_summaries",
            "outcome",
            "requirements",
            "response_rejections",
            "divergences",
            "experiment",
        },
        {
            "schema",
            "kind",
            "receipt_digest",
            "journal",
            "run",
            "skill",
            "timing",
            "operations",
            "operation_summaries",
            "outcome",
            "requirements",
            "response_rejections",
            "divergences",
        },
    )
    if receipt["schema"] != "yield.observation.v1" or receipt["kind"] != "run_receipt":
        raise ReceiptError("$: invalid schema or kind")
    _string(receipt["receipt_digest"], "$.receipt_digest", _DIGEST)

    journal = _object(
        receipt["journal"],
        "$.journal",
        {"run_id", "head_sequence", "head_digest"},
        {"run_id", "head_sequence", "head_digest"},
    )
    run_id = _string(journal["run_id"], "$.journal.run_id")
    _integer(journal["head_sequence"], "$.journal.head_sequence", 1)
    _string(journal["head_digest"], "$.journal.head_digest", _DIGEST)

    run = _object(receipt["run"], "$.run", {"id", "input_digest"}, {"id"})
    if _string(run["id"], "$.run.id") != run_id:
        raise ReceiptError("$.run.id: must match journal.run_id")
    _optional(run, "input_digest", lambda v, p: _string(v, p, _DIGEST), "$.run")

    skill = _object(
        receipt["skill"],
        "$.skill",
        {"name", "version", "binding_digest", "source_digest"},
        {"name"},
    )
    _string(skill["name"], "$.skill.name")
    _optional(skill, "version", lambda v, p: _string(v, p, _SEMVER), "$.skill")
    _optional(skill, "binding_digest", lambda v, p: _string(v, p, _DIGEST), "$.skill")
    if "source_digest" in skill:
        source = _object(
            skill["source_digest"],
            "$.skill.source_digest",
            {"profile", "value"},
            {"profile", "value"},
        )
        _string(source["profile"], "$.skill.source_digest.profile", _SOURCE_PROFILE)
        _string(source["value"], "$.skill.source_digest.value", _DIGEST)

    if "runtime" in receipt:
        runtime = _object(
            receipt["runtime"],
            "$.runtime",
            {"supervisor_version", "required_version", "compatible"},
            set(),
        )
        if not runtime:
            raise ReceiptError("$.runtime: must not be empty")
        _optional(
            runtime,
            "supervisor_version",
            lambda v, p: _string(v, p, _SEMVER),
            "$.runtime",
        )
        _optional(
            runtime,
            "required_version",
            lambda v, p: _string(v, p, _SEMVER),
            "$.runtime",
        )
        _optional(runtime, "compatible", _boolean, "$.runtime")
        if "compatible" in runtime and not {
            "supervisor_version",
            "required_version",
        }.issubset(runtime):
            raise ReceiptError("$.runtime.compatible: requires both runtime versions")

    timing = _object(
        receipt["timing"],
        "$.timing",
        {"started_at", "last_observed_at", "ended_at", "elapsed_ms", "clock_anomaly"},
        {"started_at", "last_observed_at"},
    )
    _timestamp(timing["started_at"], "$.timing.started_at")
    _timestamp(timing["last_observed_at"], "$.timing.last_observed_at")
    _optional(timing, "ended_at", _timestamp, "$.timing")
    _optional(timing, "elapsed_ms", lambda v, p: _integer(v, p, 0), "$.timing")
    _optional(timing, "clock_anomaly", _boolean, "$.timing")
    if timing.get("clock_anomaly") is False:
        raise ReceiptError("$.timing.clock_anomaly: false must be omitted")

    def operation(item: Any, path: str) -> None:
        observed = _object(
            item,
            path,
            {
                "sequence",
                "kind",
                "operation_key_digest",
                "requested_at",
                "completed_at",
                "elapsed_ms",
                "result_digest",
                "clock_anomaly",
            },
            {"sequence", "kind", "operation_key_digest", "requested_at"},
        )
        _integer(observed["sequence"], f"{path}.sequence", 1)
        _one_of(observed["kind"], f"{path}.kind", _OPERATION_KINDS)
        _string(observed["operation_key_digest"], f"{path}.operation_key_digest", _DIGEST)
        _timestamp(observed["requested_at"], f"{path}.requested_at")
        _optional(observed, "completed_at", _timestamp, path)
        _optional(observed, "elapsed_ms", lambda v, p: _integer(v, p, 0), path)
        _optional(observed, "result_digest", lambda v, p: _string(v, p, _DIGEST), path)
        _optional(observed, "clock_anomaly", _boolean, path)
        if observed.get("clock_anomaly") is False:
            raise ReceiptError(f"{path}.clock_anomaly: false must be omitted")

    _array(receipt, "operations", operation)

    def operation_summary(item: Any, path: str) -> None:
        summary = _object(
            item,
            path,
            {"kind", "requested", "completed", "total_elapsed_ms"},
            {"kind", "requested", "completed", "total_elapsed_ms"},
        )
        _one_of(summary["kind"], f"{path}.kind", _OPERATION_KINDS)
        requested = _integer(summary["requested"], f"{path}.requested", 0)
        completed = _integer(summary["completed"], f"{path}.completed", 0)
        if completed > requested:
            raise ReceiptError(f"{path}.completed: must not exceed requested")
        _integer(summary["total_elapsed_ms"], f"{path}.total_elapsed_ms", 0)

    _array(receipt, "operation_summaries", operation_summary)

    outcome = _object(
        receipt["outcome"],
        "$.outcome",
        {
            "phase",
            "terminal_disposition",
            "terminal_cause",
            "result_digest",
            "failure_code",
        },
        {"phase"},
    )
    phase = _one_of(outcome["phase"], "$.outcome.phase", _PHASES)
    _optional(
        outcome,
        "terminal_disposition",
        lambda v, p: _one_of(v, p, _DISPOSITIONS),
        "$.outcome",
    )
    _optional(
        outcome,
        "terminal_cause",
        lambda v, p: _one_of(v, p, _TERMINAL_CAUSES),
        "$.outcome",
    )
    _optional(outcome, "result_digest", lambda v, p: _string(v, p, _DIGEST), "$.outcome")
    _optional(
        outcome,
        "failure_code",
        lambda v, p: _one_of(v, p, _FAILURE_CODES),
        "$.outcome",
    )
    if phase == "terminal":
        if (
            not {"terminal_disposition", "terminal_cause"}.issubset(outcome)
            or "ended_at" not in timing
        ):
            raise ReceiptError(
                "$.outcome: terminal outcome requires disposition, cause, and end time"
            )
    elif "terminal_disposition" in outcome or "terminal_cause" in outcome:
        raise ReceiptError("$.outcome: nonterminal outcome cannot contain terminal fields")
    if phase in {"initialization_failed", "recoverable_error"} and "failure_code" not in outcome:
        raise ReceiptError("$.outcome.failure_code: required for failure phase")

    def requirement(item: Any, path: str) -> None:
        result = _object(
            item,
            path,
            {"outcome", "claim_digest", "evidence_digest"},
            {"outcome", "claim_digest"},
        )
        _one_of(result["outcome"], f"{path}.outcome", {"passed", "failed"})
        _string(result["claim_digest"], f"{path}.claim_digest", _DIGEST)
        _optional(result, "evidence_digest", lambda v, p: _string(v, p, _DIGEST), path)

    _array(receipt, "requirements", requirement)

    def rejection(item: Any, path: str) -> None:
        result = _object(item, path, {"reason", "count"}, {"reason", "count"})
        _one_of(result["reason"], f"{path}.reason", _REJECTION_REASONS)
        _integer(result["count"], f"{path}.count", 1)

    _array(receipt, "response_rejections", rejection)

    def divergence(item: Any, path: str) -> None:
        result = _object(
            item,
            path,
            {"sequence", "expected_digest", "got_digest"},
            {"sequence", "expected_digest", "got_digest"},
        )
        _integer(result["sequence"], f"{path}.sequence", 1)
        _string(result["expected_digest"], f"{path}.expected_digest", _DIGEST)
        _string(result["got_digest"], f"{path}.got_digest", _DIGEST)

    _array(receipt, "divergences", divergence)

    if "experiment" in receipt:
        experiment = _object(
            receipt["experiment"],
            "$.experiment",
            {
                "experiment_id",
                "cohort_id",
                "variant_id",
                "role",
                "baseline_variant_id",
                "parent_skill_version",
            },
            {"experiment_id", "variant_id", "role"},
        )
        _string(experiment["experiment_id"], "$.experiment.experiment_id", _IDENTIFIER)
        _optional(
            experiment,
            "cohort_id",
            lambda v, p: _string(v, p, _IDENTIFIER),
            "$.experiment",
        )
        _string(experiment["variant_id"], "$.experiment.variant_id", _IDENTIFIER)
        _one_of(experiment["role"], "$.experiment.role", {"baseline", "candidate"})
        _optional(
            experiment,
            "baseline_variant_id",
            lambda v, p: _string(v, p, _IDENTIFIER),
            "$.experiment",
        )
        _optional(
            experiment,
            "parent_skill_version",
            lambda v, p: _string(v, p, _SEMVER),
            "$.experiment",
        )


def _canonical(value: Mapping[str, Any]) -> bytes:
    try:
        return json.dumps(
            value,
            ensure_ascii=False,
            allow_nan=False,
            separators=(",", ":"),
            sort_keys=True,
        ).encode("utf-8")
    except (TypeError, ValueError) as error:
        raise ReceiptError(f"receipt contains a non-JSON value: {error}") from error


def _object_pairs(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    value: dict[str, Any] = {}
    for key, item in pairs:
        if key in value:
            raise ReceiptError(f"duplicate field {key!r}")
        value[key] = item
    return value


def verify_run_receipt(receipt: RunReceipt, canonical: bytes) -> None:
    """Verify a typed receipt against its exact canonical bytes and digest."""
    _validate_receipt(receipt)
    encoded = _canonical(receipt)
    if encoded != canonical:
        raise ReceiptError("receipt bytes are not canonical JSON")
    body = dict(receipt)
    expected = cast(str, body.pop("receipt_digest"))
    actual = "sha256:" + hashlib.sha256(_canonical(body)).hexdigest()
    if actual != expected:
        raise ReceiptError("receipt digest verification failed")


def parse_run_receipt(canonical: bytes) -> RunReceipt:
    """Parse and verify one canonical yield.observation.v1 receipt."""
    try:
        document = json.loads(canonical.decode("utf-8"), object_pairs_hook=_object_pairs)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ReceiptError(f"receipt does not decode: {error}") from error
    _validate_receipt(document)
    receipt = cast(RunReceipt, document)
    verify_run_receipt(receipt, canonical)
    return receipt
