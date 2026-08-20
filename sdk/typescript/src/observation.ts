import { createHash } from "node:crypto"

export type ReceiptOperationKind = "ask_user" | "agent_task" | "run_command"
export type LifecyclePhase =
  | "initializing"
  | "initialization_failed"
  | "awaiting_response"
  | "advancing"
  | "recoverable_error"
  | "diverged"
  | "terminal"

export interface JournalBinding {
  run_id: string
  head_sequence: number
  head_digest: string
}

export interface RunIdentity {
  id: string
  input_digest?: string
}

export interface ProfileDigest {
  profile: string
  value: string
}

export interface ReceiptSkillIdentity {
  name: string
  version?: string
  binding_digest?: string
  source_digest?: ProfileDigest
}

export interface RuntimeIdentity {
  supervisor_version?: string
  required_version?: string
  compatible?: boolean
}

export interface TimingSummary {
  started_at: string
  last_observed_at: string
  ended_at?: string
  elapsed_ms?: number
  clock_anomaly?: boolean
}

export interface OperationObservation {
  sequence: number
  kind: ReceiptOperationKind
  operation_key_digest: string
  requested_at: string
  completed_at?: string
  elapsed_ms?: number
  result_digest?: string
  clock_anomaly?: boolean
}

export interface OperationSummary {
  kind: ReceiptOperationKind
  requested: number
  completed: number
  total_elapsed_ms: number
}

export interface OutcomeSummary {
  phase: LifecyclePhase
  terminal_disposition?: "completed" | "blocked" | "refused"
  terminal_cause?:
    "completed" | "blocked" | "refused" | "requirement_failed" | "completion_unproven"
  result_digest?: string
  failure_code?: FailureCode
}

export type FailureCode =
  | "manifest_invalid"
  | "manifest_read_failed"
  | "runtime_version_missing"
  | "runtime_incompatible"
  | "runner_missing"
  | "source_digest_failed"
  | "initialization_failed"
  | "invalid_program_output"
  | "execution_timeout"
  | "subprocess_failed"
  | "execution_failed"
  | "command_execution_failed"

export interface RequirementOutcome {
  outcome: "passed" | "failed"
  claim_digest: string
  evidence_digest?: string
}

export interface ResponseRejectionSummary {
  reason:
    | "wrong-run"
    | "stale-response"
    | "duplicate-response"
    | "wrong-request"
    | "schema-invalid"
    | "digest-mismatch"
    | "completion-unproven"
    | "run-closed"
    | "no-pending-operation"
  count: number
}

export interface DivergenceOutcome {
  sequence: number
  expected_digest: string
  got_digest: string
}

export interface ExperimentContext {
  experiment_id: string
  cohort_id?: string
  variant_id: string
  role: "baseline" | "candidate"
  baseline_variant_id?: string
  parent_skill_version?: string
}

export interface RunReceipt {
  schema: "yield.observation.v1"
  kind: "run_receipt"
  receipt_digest: string
  journal: JournalBinding
  run: RunIdentity
  skill: ReceiptSkillIdentity
  runtime?: RuntimeIdentity
  timing: TimingSummary
  operations: OperationObservation[]
  operation_summaries: OperationSummary[]
  outcome: OutcomeSummary
  requirements: RequirementOutcome[]
  response_rejections: ResponseRejectionSummary[]
  divergences: DivergenceOutcome[]
  experiment?: ExperimentContext
}

export class ReceiptError extends Error {}

type RecordValue = Record<string, unknown>

const digestPattern = /^sha256:[0-9a-f]{64}$/
const semverPattern = /^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/
const sourceProfilePattern = /^yield\.skill-source\.v[0-9]+$/
const identifierPattern = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/
const timestampPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/

const operationKinds = ["ask_user", "agent_task", "run_command"] as const
const phases = [
  "initializing",
  "initialization_failed",
  "awaiting_response",
  "advancing",
  "recoverable_error",
  "diverged",
  "terminal",
] as const
const dispositions = ["completed", "blocked", "refused"] as const
const terminalCauses = [
  "completed",
  "blocked",
  "refused",
  "requirement_failed",
  "completion_unproven",
] as const
const failureCodes = [
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
] as const
const rejectionReasons = [
  "wrong-run",
  "stale-response",
  "duplicate-response",
  "wrong-request",
  "schema-invalid",
  "digest-mismatch",
  "completion-unproven",
  "run-closed",
  "no-pending-operation",
] as const

function fail(path: string, message: string): never {
  throw new ReceiptError(`${path}: ${message}`)
}

function object(
  value: unknown,
  path: string,
  allowed: readonly string[],
  required: readonly string[],
): RecordValue {
  if (value === null || typeof value !== "object" || Array.isArray(value))
    fail(path, "must be an object")
  const record = value as RecordValue
  for (const key of Object.keys(record))
    if (!allowed.includes(key)) fail(path, `unknown field ${JSON.stringify(key)}`)
  for (const key of required)
    if (!Object.hasOwn(record, key)) fail(path, `missing field ${JSON.stringify(key)}`)
  return record
}

function string(value: unknown, path: string, pattern?: RegExp): string {
  if (
    typeof value !== "string" ||
    value.length === 0 ||
    (pattern && !pattern.test(value)) ||
    [...value].some((character) => {
      const point = character.codePointAt(0)!
      return point >= 0xd800 && point <= 0xdfff
    })
  )
    fail(path, "must be a valid non-empty string")
  return value
}

function timestamp(value: unknown, path: string): string {
  const text = string(value, path, timestampPattern)
  const match = text.match(
    /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|[+-](\d{2}):(\d{2}))$/,
  )!
  const [year, month, day, hour, minute, second, offsetHour, offsetMinute] = match
    .slice(1)
    .map((part) => (part === undefined ? 0 : Number(part)))
  const leap = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0)
  const days = [0, 31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
  if (
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > days[month] ||
    hour > 23 ||
    minute > 59 ||
    second > 59 ||
    offsetHour > 23 ||
    offsetMinute > 59
  )
    fail(path, "must be an RFC 3339 timestamp")
  return text
}

function integer(value: unknown, path: string, minimum: number): number {
  if (!Number.isSafeInteger(value) || (value as number) < minimum)
    fail(path, `must be an integer >= ${minimum}`)
  return value as number
}

function boolean(value: unknown, path: string): boolean {
  if (typeof value !== "boolean") fail(path, "must be a boolean")
  return value
}

function oneOf<const T extends readonly string[]>(
  value: unknown,
  path: string,
  values: T,
): T[number] {
  if (typeof value !== "string" || !values.includes(value))
    fail(path, `must be one of ${values.join(", ")}`)
  return value as T[number]
}

function optional(
  record: RecordValue,
  key: string,
  validate: (value: unknown, path: string) => unknown,
  path: string,
): void {
  if (Object.hasOwn(record, key)) validate(record[key], `${path}.${key}`)
}

function array(
  record: RecordValue,
  key: string,
  validate: (value: unknown, path: string) => void,
  path: string,
): void {
  const value = record[key]
  if (!Array.isArray(value)) fail(`${path}.${key}`, "must be an array")
  value.forEach((item, index) => validate(item, `${path}.${key}[${index}]`))
}

function validateReceipt(value: unknown): asserts value is RunReceipt {
  const receipt = object(
    value,
    "$",
    [
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
    ],
    [
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
    ],
  )
  if (receipt.schema !== "yield.observation.v1" || receipt.kind !== "run_receipt")
    fail("$", "invalid schema or kind")
  string(receipt.receipt_digest, "$.receipt_digest", digestPattern)

  const journal = object(
    receipt.journal,
    "$.journal",
    ["run_id", "head_sequence", "head_digest"],
    ["run_id", "head_sequence", "head_digest"],
  )
  const runID = string(journal.run_id, "$.journal.run_id")
  integer(journal.head_sequence, "$.journal.head_sequence", 1)
  string(journal.head_digest, "$.journal.head_digest", digestPattern)

  const run = object(receipt.run, "$.run", ["id", "input_digest"], ["id"])
  if (string(run.id, "$.run.id") !== runID) fail("$.run.id", "must match journal.run_id")
  optional(run, "input_digest", (v, p) => string(v, p, digestPattern), "$.run")

  const skill = object(
    receipt.skill,
    "$.skill",
    ["name", "version", "binding_digest", "source_digest"],
    ["name"],
  )
  string(skill.name, "$.skill.name")
  optional(skill, "version", (v, p) => string(v, p, semverPattern), "$.skill")
  optional(skill, "binding_digest", (v, p) => string(v, p, digestPattern), "$.skill")
  if (Object.hasOwn(skill, "source_digest")) {
    const source = object(
      skill.source_digest,
      "$.skill.source_digest",
      ["profile", "value"],
      ["profile", "value"],
    )
    string(source.profile, "$.skill.source_digest.profile", sourceProfilePattern)
    string(source.value, "$.skill.source_digest.value", digestPattern)
  }

  if (Object.hasOwn(receipt, "runtime")) {
    const runtime = object(
      receipt.runtime,
      "$.runtime",
      ["supervisor_version", "required_version", "compatible"],
      [],
    )
    if (Object.keys(runtime).length === 0) fail("$.runtime", "must not be empty")
    optional(runtime, "supervisor_version", (v, p) => string(v, p, semverPattern), "$.runtime")
    optional(runtime, "required_version", (v, p) => string(v, p, semverPattern), "$.runtime")
    optional(runtime, "compatible", boolean, "$.runtime")
    if (
      Object.hasOwn(runtime, "compatible") &&
      (!Object.hasOwn(runtime, "supervisor_version") || !Object.hasOwn(runtime, "required_version"))
    )
      fail("$.runtime.compatible", "requires both runtime versions")
  }

  const timing = object(
    receipt.timing,
    "$.timing",
    ["started_at", "last_observed_at", "ended_at", "elapsed_ms", "clock_anomaly"],
    ["started_at", "last_observed_at"],
  )
  timestamp(timing.started_at, "$.timing.started_at")
  timestamp(timing.last_observed_at, "$.timing.last_observed_at")
  optional(timing, "ended_at", timestamp, "$.timing")
  optional(timing, "elapsed_ms", (v, p) => integer(v, p, 0), "$.timing")
  optional(timing, "clock_anomaly", boolean, "$.timing")
  if (timing.clock_anomaly === false)
    fail("$.timing.clock_anomaly", "false must be omitted from canonical receipts")

  array(
    receipt,
    "operations",
    (item, path) => {
      const operation = object(
        item,
        path,
        [
          "sequence",
          "kind",
          "operation_key_digest",
          "requested_at",
          "completed_at",
          "elapsed_ms",
          "result_digest",
          "clock_anomaly",
        ],
        ["sequence", "kind", "operation_key_digest", "requested_at"],
      )
      integer(operation.sequence, `${path}.sequence`, 1)
      oneOf(operation.kind, `${path}.kind`, operationKinds)
      string(operation.operation_key_digest, `${path}.operation_key_digest`, digestPattern)
      timestamp(operation.requested_at, `${path}.requested_at`)
      optional(operation, "completed_at", timestamp, path)
      optional(operation, "elapsed_ms", (v, p) => integer(v, p, 0), path)
      optional(operation, "result_digest", (v, p) => string(v, p, digestPattern), path)
      optional(operation, "clock_anomaly", boolean, path)
      if (operation.clock_anomaly === false)
        fail(`${path}.clock_anomaly`, "false must be omitted from canonical receipts")
    },
    "$",
  )

  array(
    receipt,
    "operation_summaries",
    (item, path) => {
      const summary = object(
        item,
        path,
        ["kind", "requested", "completed", "total_elapsed_ms"],
        ["kind", "requested", "completed", "total_elapsed_ms"],
      )
      oneOf(summary.kind, `${path}.kind`, operationKinds)
      const requested = integer(summary.requested, `${path}.requested`, 0)
      const completed = integer(summary.completed, `${path}.completed`, 0)
      if (completed > requested) fail(`${path}.completed`, "must not exceed requested")
      integer(summary.total_elapsed_ms, `${path}.total_elapsed_ms`, 0)
    },
    "$",
  )

  const outcome = object(
    receipt.outcome,
    "$.outcome",
    ["phase", "terminal_disposition", "terminal_cause", "result_digest", "failure_code"],
    ["phase"],
  )
  const phase = oneOf(outcome.phase, "$.outcome.phase", phases)
  optional(outcome, "terminal_disposition", (v, p) => oneOf(v, p, dispositions), "$.outcome")
  optional(outcome, "terminal_cause", (v, p) => oneOf(v, p, terminalCauses), "$.outcome")
  optional(outcome, "result_digest", (v, p) => string(v, p, digestPattern), "$.outcome")
  optional(outcome, "failure_code", (v, p) => oneOf(v, p, failureCodes), "$.outcome")
  if (phase === "terminal") {
    if (
      !Object.hasOwn(outcome, "terminal_disposition") ||
      !Object.hasOwn(outcome, "terminal_cause") ||
      !Object.hasOwn(timing, "ended_at")
    )
      fail("$.outcome", "terminal outcome requires disposition, cause, and end time")
  } else if (
    Object.hasOwn(outcome, "terminal_disposition") ||
    Object.hasOwn(outcome, "terminal_cause")
  )
    fail("$.outcome", "nonterminal outcome cannot contain terminal fields")
  if (
    (phase === "initialization_failed" || phase === "recoverable_error") &&
    !Object.hasOwn(outcome, "failure_code")
  )
    fail("$.outcome.failure_code", "required for failure phase")

  array(
    receipt,
    "requirements",
    (item, path) => {
      const requirement = object(
        item,
        path,
        ["outcome", "claim_digest", "evidence_digest"],
        ["outcome", "claim_digest"],
      )
      oneOf(requirement.outcome, `${path}.outcome`, ["passed", "failed"] as const)
      string(requirement.claim_digest, `${path}.claim_digest`, digestPattern)
      optional(requirement, "evidence_digest", (v, p) => string(v, p, digestPattern), path)
    },
    "$",
  )

  array(
    receipt,
    "response_rejections",
    (item, path) => {
      const rejection = object(item, path, ["reason", "count"], ["reason", "count"])
      oneOf(rejection.reason, `${path}.reason`, rejectionReasons)
      integer(rejection.count, `${path}.count`, 1)
    },
    "$",
  )

  array(
    receipt,
    "divergences",
    (item, path) => {
      const divergence = object(
        item,
        path,
        ["sequence", "expected_digest", "got_digest"],
        ["sequence", "expected_digest", "got_digest"],
      )
      integer(divergence.sequence, `${path}.sequence`, 1)
      string(divergence.expected_digest, `${path}.expected_digest`, digestPattern)
      string(divergence.got_digest, `${path}.got_digest`, digestPattern)
    },
    "$",
  )

  if (Object.hasOwn(receipt, "experiment")) {
    const experiment = object(
      receipt.experiment,
      "$.experiment",
      [
        "experiment_id",
        "cohort_id",
        "variant_id",
        "role",
        "baseline_variant_id",
        "parent_skill_version",
      ],
      ["experiment_id", "variant_id", "role"],
    )
    string(experiment.experiment_id, "$.experiment.experiment_id", identifierPattern)
    optional(experiment, "cohort_id", (v, p) => string(v, p, identifierPattern), "$.experiment")
    string(experiment.variant_id, "$.experiment.variant_id", identifierPattern)
    oneOf(experiment.role, "$.experiment.role", ["baseline", "candidate"] as const)
    optional(
      experiment,
      "baseline_variant_id",
      (v, p) => string(v, p, identifierPattern),
      "$.experiment",
    )
    optional(
      experiment,
      "parent_skill_version",
      (v, p) => string(v, p, semverPattern),
      "$.experiment",
    )
  }
}

function canonical(value: unknown): string {
  if (value === null) return "null"
  if (typeof value === "boolean") return value ? "true" : "false"
  if (typeof value === "string") return JSON.stringify(value)
  if (typeof value === "number") {
    if (!Number.isSafeInteger(value))
      throw new ReceiptError("receipt contains a non-integer or unsafe JSON number")
    return String(value)
  }
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`
  if (typeof value === "object") {
    const record = value as RecordValue
    return `{${Object.keys(record)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${canonical(record[key])}`)
      .join(",")}}`
  }
  throw new ReceiptError("receipt contains a non-JSON value")
}

function rawBytes(bytes: Uint8Array | string): Uint8Array {
  return typeof bytes === "string" ? Buffer.from(bytes, "utf8") : bytes
}

/** Verify a typed receipt against its exact canonical bytes and digest. */
export function verifyRunReceipt(receipt: RunReceipt, bytes: Uint8Array | string): void {
  validateReceipt(receipt)
  const canonicalReceipt = Buffer.from(canonical(receipt), "utf8")
  if (!Buffer.from(rawBytes(bytes)).equals(canonicalReceipt))
    throw new ReceiptError("receipt bytes are not canonical JSON")
  const body = { ...receipt } as RecordValue
  delete body.receipt_digest
  const digest = `sha256:${createHash("sha256").update(canonical(body)).digest("hex")}`
  if (digest !== receipt.receipt_digest)
    throw new ReceiptError("receipt digest verification failed")
}

/** Parse and verify one canonical yield.observation.v1 receipt. */
export function parseRunReceipt(bytes: Uint8Array | string): RunReceipt {
  const raw = rawBytes(bytes)
  let decoded: unknown
  try {
    decoded = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(raw))
  } catch (error) {
    throw new ReceiptError(`receipt does not decode: ${String(error)}`)
  }
  validateReceipt(decoded)
  verifyRunReceipt(decoded, raw)
  return decoded
}
