//! Strict types and verification for `yield.observation.v1` run receipts.

use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::fmt::{Display, Formatter};

#[derive(Debug)]
pub struct ReceiptError(String);

impl ReceiptError {
    fn new(message: impl Into<String>) -> Self {
        Self(message.into())
    }
}

impl Display for ReceiptError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.0)
    }
}

impl std::error::Error for ReceiptError {}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OperationKind {
    AskUser,
    AgentTask,
    RunCommand,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum LifecyclePhase {
    Initializing,
    InitializationFailed,
    AwaitingResponse,
    Advancing,
    RecoverableError,
    Diverged,
    Terminal,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TerminalDisposition {
    Completed,
    Blocked,
    Refused,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TerminalCause {
    Completed,
    Blocked,
    Refused,
    RequirementFailed,
    CompletionUnproven,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum FailureCode {
    ManifestInvalid,
    ManifestReadFailed,
    RuntimeVersionMissing,
    RuntimeIncompatible,
    RunnerMissing,
    SourceDigestFailed,
    InitializationFailed,
    InvalidProgramOutput,
    ExecutionTimeout,
    SubprocessFailed,
    ExecutionFailed,
    CommandExecutionFailed,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RequirementResult {
    Passed,
    Failed,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum ResponseRejectionReason {
    WrongRun,
    StaleResponse,
    DuplicateResponse,
    WrongRequest,
    SchemaInvalid,
    DigestMismatch,
    CompletionUnproven,
    RunClosed,
    NoPendingOperation,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ExperimentRole {
    Baseline,
    Candidate,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct JournalBinding {
    pub run_id: String,
    pub head_sequence: u64,
    pub head_digest: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RunIdentity {
    pub id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_digest: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ProfileDigest {
    pub profile: String,
    pub value: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SkillIdentity {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub binding_digest: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_digest: Option<ProfileDigest>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RuntimeIdentity {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub supervisor_version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub required_version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub compatible: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TimingSummary {
    pub started_at: String,
    pub last_observed_at: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ended_at: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub elapsed_ms: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub clock_anomaly: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct OperationObservation {
    pub sequence: u64,
    pub kind: OperationKind,
    pub operation_key_digest: String,
    pub requested_at: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub completed_at: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub elapsed_ms: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result_digest: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub clock_anomaly: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct OperationSummary {
    pub kind: OperationKind,
    pub requested: u64,
    pub completed: u64,
    pub total_elapsed_ms: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct OutcomeSummary {
    pub phase: LifecyclePhase,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub terminal_disposition: Option<TerminalDisposition>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub terminal_cause: Option<TerminalCause>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result_digest: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub failure_code: Option<FailureCode>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RequirementOutcome {
    pub outcome: RequirementResult,
    pub claim_digest: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub evidence_digest: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ResponseRejectionSummary {
    pub reason: ResponseRejectionReason,
    pub count: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct DivergenceOutcome {
    pub sequence: u64,
    pub expected_digest: String,
    pub got_digest: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ExperimentContext {
    pub experiment_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cohort_id: Option<String>,
    pub variant_id: String,
    pub role: ExperimentRole,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub baseline_variant_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parent_skill_version: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RunReceipt {
    pub schema: String,
    pub kind: String,
    pub receipt_digest: String,
    pub journal: JournalBinding,
    pub run: RunIdentity,
    pub skill: SkillIdentity,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub runtime: Option<RuntimeIdentity>,
    pub timing: TimingSummary,
    pub operations: Vec<OperationObservation>,
    pub operation_summaries: Vec<OperationSummary>,
    pub outcome: OutcomeSummary,
    pub requirements: Vec<RequirementOutcome>,
    pub response_rejections: Vec<ResponseRejectionSummary>,
    pub divergences: Vec<DivergenceOutcome>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub experiment: Option<ExperimentContext>,
}

impl RunReceipt {
    /// Parse and verify one canonical `yield.observation.v1` receipt.
    pub fn parse_and_verify(bytes: &[u8]) -> Result<Self, ReceiptError> {
        let receipt: Self = serde_json::from_slice(bytes)
            .map_err(|error| ReceiptError::new(format!("receipt does not decode: {error}")))?;
        receipt.verify(bytes)?;
        Ok(receipt)
    }

    /// Verify this typed receipt against its exact canonical bytes and digest.
    pub fn verify(&self, bytes: &[u8]) -> Result<(), ReceiptError> {
        self.validate()?;
        let value = serde_json::to_value(self)
            .map_err(|error| ReceiptError::new(format!("receipt does not encode: {error}")))?;
        let canonical = canonical_bytes(&value)?;
        if canonical != bytes {
            return Err(ReceiptError::new("receipt bytes are not canonical JSON"));
        }
        let mut body = value;
        let Value::Object(ref mut object) = body else {
            return Err(ReceiptError::new("receipt must be an object"));
        };
        object.remove("receipt_digest");
        let actual = format!(
            "sha256:{}",
            hex::encode(Sha256::digest(canonical_bytes(&body)?))
        );
        if actual != self.receipt_digest {
            return Err(ReceiptError::new("receipt digest verification failed"));
        }
        Ok(())
    }

    fn validate(&self) -> Result<(), ReceiptError> {
        if self.schema != "yield.observation.v1" || self.kind != "run_receipt" {
            return Err(ReceiptError::new("invalid receipt schema or kind"));
        }
        require_digest(&self.receipt_digest, "receipt_digest")?;
        if self.journal.run_id.is_empty() || self.journal.head_sequence == 0 {
            return Err(ReceiptError::new("incomplete journal binding"));
        }
        require_digest(&self.journal.head_digest, "journal.head_digest")?;
        if self.run.id != self.journal.run_id || self.skill.name.is_empty() {
            return Err(ReceiptError::new("incomplete run or skill identity"));
        }
        optional_digest(self.run.input_digest.as_deref(), "run.input_digest")?;
        optional_digest(self.skill.binding_digest.as_deref(), "skill.binding_digest")?;
        if let Some(version) = &self.skill.version {
            require_semver(version, "skill.version")?;
        }
        if let Some(source) = &self.skill.source_digest {
            if !source_profile(&source.profile) {
                return Err(ReceiptError::new("invalid skill.source_digest.profile"));
            }
            require_digest(&source.value, "skill.source_digest.value")?;
        }
        if let Some(runtime) = &self.runtime {
            if runtime.supervisor_version.is_none()
                && runtime.required_version.is_none()
                && runtime.compatible.is_none()
            {
                return Err(ReceiptError::new("runtime identity must not be empty"));
            }
            if let Some(version) = &runtime.supervisor_version {
                require_semver(version, "runtime.supervisor_version")?;
            }
            if let Some(version) = &runtime.required_version {
                require_semver(version, "runtime.required_version")?;
            }
            if runtime.compatible.is_some()
                && (runtime.supervisor_version.is_none() || runtime.required_version.is_none())
            {
                return Err(ReceiptError::new(
                    "runtime.compatible requires both versions",
                ));
            }
        }
        require_timestamp(&self.timing.started_at, "timing.started_at")?;
        require_timestamp(&self.timing.last_observed_at, "timing.last_observed_at")?;
        if let Some(value) = &self.timing.ended_at {
            require_timestamp(value, "timing.ended_at")?;
        }
        if self.timing.clock_anomaly == Some(false) {
            return Err(ReceiptError::new(
                "timing.clock_anomaly false must be omitted",
            ));
        }
        for (index, operation) in self.operations.iter().enumerate() {
            if operation.sequence == 0 {
                return Err(ReceiptError::new(format!(
                    "operations[{index}].sequence must be positive"
                )));
            }
            require_digest(
                &operation.operation_key_digest,
                "operation.operation_key_digest",
            )?;
            require_timestamp(&operation.requested_at, "operation.requested_at")?;
            if let Some(value) = &operation.completed_at {
                require_timestamp(value, "operation.completed_at")?;
            }
            optional_digest(
                operation.result_digest.as_deref(),
                "operation.result_digest",
            )?;
            if operation.clock_anomaly == Some(false) {
                return Err(ReceiptError::new(
                    "operation.clock_anomaly false must be omitted",
                ));
            }
        }
        for summary in &self.operation_summaries {
            if summary.completed > summary.requested {
                return Err(ReceiptError::new(
                    "operation summary completed exceeds requested",
                ));
            }
        }
        if self.outcome.phase == LifecyclePhase::Terminal {
            if self.outcome.terminal_disposition.is_none()
                || self.outcome.terminal_cause.is_none()
                || self.timing.ended_at.is_none()
            {
                return Err(ReceiptError::new("terminal outcome is incomplete"));
            }
        } else if self.outcome.terminal_disposition.is_some()
            || self.outcome.terminal_cause.is_some()
        {
            return Err(ReceiptError::new(
                "nonterminal outcome contains terminal fields",
            ));
        }
        if matches!(
            self.outcome.phase,
            LifecyclePhase::InitializationFailed | LifecyclePhase::RecoverableError
        ) && self.outcome.failure_code.is_none()
        {
            return Err(ReceiptError::new("failure phase has no failure_code"));
        }
        optional_digest(
            self.outcome.result_digest.as_deref(),
            "outcome.result_digest",
        )?;
        for requirement in &self.requirements {
            require_digest(&requirement.claim_digest, "requirement.claim_digest")?;
            optional_digest(
                requirement.evidence_digest.as_deref(),
                "requirement.evidence_digest",
            )?;
        }
        for rejection in &self.response_rejections {
            if rejection.count == 0 {
                return Err(ReceiptError::new(
                    "response rejection count must be positive",
                ));
            }
        }
        for divergence in &self.divergences {
            if divergence.sequence == 0 {
                return Err(ReceiptError::new("divergence sequence must be positive"));
            }
            require_digest(&divergence.expected_digest, "divergence.expected_digest")?;
            require_digest(&divergence.got_digest, "divergence.got_digest")?;
        }
        if let Some(experiment) = &self.experiment {
            require_identifier(&experiment.experiment_id, "experiment.experiment_id")?;
            require_identifier(&experiment.variant_id, "experiment.variant_id")?;
            if let Some(value) = &experiment.cohort_id {
                require_identifier(value, "experiment.cohort_id")?;
            }
            if let Some(value) = &experiment.baseline_variant_id {
                require_identifier(value, "experiment.baseline_variant_id")?;
            }
            if let Some(value) = &experiment.parent_skill_version {
                require_semver(value, "experiment.parent_skill_version")?;
            }
        }
        Ok(())
    }
}

fn require_digest(value: &str, field: &str) -> Result<(), ReceiptError> {
    let valid = value.len() == 71
        && value.starts_with("sha256:")
        && value[7..]
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte));
    if valid {
        Ok(())
    } else {
        Err(ReceiptError::new(format!("invalid {field}")))
    }
}

fn optional_digest(value: Option<&str>, field: &str) -> Result<(), ReceiptError> {
    match value {
        Some(value) => require_digest(value, field),
        None => Ok(()),
    }
}

fn require_semver(value: &str, field: &str) -> Result<(), ReceiptError> {
    let split = value.find(['-', '+']).unwrap_or(value.len());
    let core = &value[..split];
    let suffix = &value[split..];
    let valid_core = core.split('.').count() == 3
        && core
            .split('.')
            .all(|part| !part.is_empty() && part.bytes().all(|byte| byte.is_ascii_digit()));
    let valid_suffix = suffix.is_empty()
        || suffix.len() > 1
            && suffix[1..]
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'.' | b'-'));
    if valid_core && valid_suffix {
        Ok(())
    } else {
        Err(ReceiptError::new(format!("invalid {field}")))
    }
}

fn source_profile(value: &str) -> bool {
    value
        .strip_prefix("yield.skill-source.v")
        .is_some_and(|version| {
            !version.is_empty() && version.bytes().all(|byte| byte.is_ascii_digit())
        })
}

fn require_identifier(value: &str, field: &str) -> Result<(), ReceiptError> {
    let valid = !value.is_empty()
        && value.len() <= 128
        && value.as_bytes()[0].is_ascii_alphanumeric()
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'.' | b'_' | b':' | b'-'));
    if valid {
        Ok(())
    } else {
        Err(ReceiptError::new(format!("invalid {field}")))
    }
}

fn require_timestamp(value: &str, field: &str) -> Result<(), ReceiptError> {
    let bytes = value.as_bytes();
    let fixed = bytes.len() >= 20
        && bytes.get(4) == Some(&b'-')
        && bytes.get(7) == Some(&b'-')
        && bytes.get(10) == Some(&b'T')
        && bytes.get(13) == Some(&b':')
        && bytes.get(16) == Some(&b':');
    let zone_start = if value.ends_with('Z') {
        value.len() - 1
    } else if value.len() >= 6
        && matches!(bytes[value.len() - 6], b'+' | b'-')
        && bytes[value.len() - 3] == b':'
    {
        value.len() - 6
    } else {
        0
    };
    if !fixed || zone_start < 19 {
        return Err(ReceiptError::new(format!("invalid {field}")));
    }
    let parse = |start: usize, end: usize| {
        value[start..end]
            .parse::<u32>()
            .map_err(|_| ReceiptError::new(format!("invalid {field}")))
    };
    let year = parse(0, 4)?;
    let month = parse(5, 7)?;
    let day = parse(8, 10)?;
    let hour = parse(11, 13)?;
    let minute = parse(14, 16)?;
    let second = parse(17, 19)?;
    let leap = year % 4 == 0 && (year % 100 != 0 || year % 400 == 0);
    let days = match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if leap => 29,
        2 => 28,
        _ => 0,
    };
    if day == 0 || day > days || hour > 23 || minute > 59 || second > 59 {
        return Err(ReceiptError::new(format!("invalid {field}")));
    }
    if !value.ends_with('Z') {
        let offset_hour = parse(value.len() - 5, value.len() - 3)?;
        let offset_minute = parse(value.len() - 2, value.len())?;
        if offset_hour > 23 || offset_minute > 59 {
            return Err(ReceiptError::new(format!("invalid {field}")));
        }
    }
    let fraction = &value[19..zone_start];
    let valid_fraction = fraction.is_empty()
        || fraction.starts_with('.')
            && fraction.len() <= 10
            && fraction[1..].bytes().all(|byte| byte.is_ascii_digit());
    if valid_fraction {
        Ok(())
    } else {
        Err(ReceiptError::new(format!("invalid {field}")))
    }
}

fn canonical_bytes(value: &Value) -> Result<Vec<u8>, ReceiptError> {
    let mut output = String::new();
    write_canonical(&mut output, value)?;
    Ok(output.into_bytes())
}

fn write_canonical(output: &mut String, value: &Value) -> Result<(), ReceiptError> {
    match value {
        Value::Null => output.push_str("null"),
        Value::Bool(value) => output.push_str(if *value { "true" } else { "false" }),
        Value::String(value) => output.push_str(
            &serde_json::to_string(value)
                .map_err(|error| ReceiptError::new(format!("string does not encode: {error}")))?,
        ),
        Value::Number(value) => {
            if !value.is_i64() && !value.is_u64() {
                return Err(ReceiptError::new("receipt contains a non-integer number"));
            }
            output.push_str(&value.to_string());
        }
        Value::Array(values) => {
            output.push('[');
            for (index, value) in values.iter().enumerate() {
                if index > 0 {
                    output.push(',');
                }
                write_canonical(output, value)?;
            }
            output.push(']');
        }
        Value::Object(values) => {
            let mut keys = values.keys().collect::<Vec<_>>();
            keys.sort_by_key(|key| key.encode_utf16().collect::<Vec<_>>());
            output.push('{');
            for (index, key) in keys.iter().enumerate() {
                if index > 0 {
                    output.push(',');
                }
                output.push_str(
                    &serde_json::to_string(key).map_err(|error| {
                        ReceiptError::new(format!("key does not encode: {error}"))
                    })?,
                );
                output.push(':');
                write_canonical(output, &values[*key])?;
            }
            output.push('}');
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    const FIXTURE_FILE: &[u8] =
        include_bytes!("../../../ir/yield.observation.v1/testdata/run-receipt.canonical.jsonl");

    fn fixture() -> &'static [u8] {
        FIXTURE_FILE
            .strip_suffix(b"\n")
            .expect("fixture file newline")
    }

    #[test]
    fn parses_and_verifies_shared_go_canonical_receipt() {
        let receipt = RunReceipt::parse_and_verify(fixture()).expect("valid receipt");
        assert_eq!(
            receipt.receipt_digest,
            "sha256:a0995d74d51e472f61dc2001f82b93fd8086f9c2f2816a22bb7e5c7fa3c01cdd"
        );
        assert_eq!(receipt.operations[0].kind, OperationKind::AgentTask);
        receipt.verify(fixture()).expect("verified receipt");
    }

    #[test]
    fn rejects_unknown_noncanonical_and_digest_mismatch() {
        let mut unknown = fixture()[..fixture().len() - 1].to_vec();
        unknown.extend_from_slice(br#","prompt":"secret"}"#);
        assert!(RunReceipt::parse_and_verify(&unknown).is_err());

        let mut noncanonical = fixture().to_vec();
        noncanonical.push(b'\n');
        assert!(RunReceipt::parse_and_verify(&noncanonical).is_err());

        let current = b"a0995d74d51e472f61dc2001f82b93fd8086f9c2f2816a22bb7e5c7fa3c01cdd";
        let position = fixture()
            .windows(current.len())
            .position(|window| window == current)
            .expect("digest");
        let mut mismatch = fixture().to_vec();
        mismatch[position..position + current.len()].fill(b'0');
        let error = RunReceipt::parse_and_verify(&mismatch).expect_err("digest mismatch");
        assert!(error.to_string().contains("digest"));
    }
}
