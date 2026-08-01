//! Yield skill-program SDK for Rust (yield.v1).
//!
//! Implements the Locus-certified SDK execution contract (see ir/README.md):
//! load the journal, replay recorded operations with a digest comparison at
//! EVERY replayed step before consuming its response, emit exactly one
//! program output (request | terminal | diverged) on stdout, then exit.
//!
//! Programs must be deterministic between yields: same journal, same
//! operations, every execution. Clocks, RNGs, environment reads, and
//! filesystem state are side effects — cross them through a yielded
//! operation or leave them out.
//!
//! The crate is named `yieldskill` because `yield` is a Rust keyword.

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::io::Write;
use std::process::exit;

pub const PROTOCOL: &str = "yield.v1";

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SkillRef {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    pub digest: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Request {
    pub id: String,
    pub kind: String,
    pub payload: Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_schema: Option<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RequestEnvelope {
    pub protocol: String,
    pub run_id: String,
    pub skill: SkillRef,
    pub sequence: u64,
    pub request: Request,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseEnvelope {
    pub run_id: String,
    pub sequence: u64,
    pub request_id: String,
    pub status: String,
    pub result: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JournalEntry {
    pub request: Request,
    pub response: ResponseEnvelope,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Journal {
    pub run_id: String,
    pub skill: SkillRef,
    #[serde(default)]
    pub entries: Vec<JournalEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommandResult {
    pub exit_code: i64,
    pub stdout: String,
    pub stderr: String,
    #[serde(default)]
    pub timed_out: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Requirement {
    pub claim: String,
    pub passed: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub evidence_digest: Option<String>,
}

#[derive(Debug, Serialize)]
struct TerminalOutcome {
    status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    reason: Option<String>,
}

#[derive(Debug, Serialize)]
struct Divergence {
    sequence: u64,
    expected_digest: String,
    got_digest: String,
    detail: String,
}

#[derive(Debug, Serialize)]
struct ProgramOutput {
    #[serde(rename = "type")]
    kind: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    envelope: Option<RequestEnvelope>,
    #[serde(skip_serializing_if = "Option::is_none")]
    terminal: Option<TerminalOutcome>,
    #[serde(skip_serializing_if = "Option::is_none")]
    divergence: Option<Divergence>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    requirements: Vec<Requirement>,
}

/// The honest terminal exits. A failed `require` never reaches program
/// code — the SDK emits and exits directly.
#[derive(Debug)]
pub enum Exit {
    /// A true frontier was reached — say so explicitly.
    Blocked(String),
    /// The skill declines to proceed, with a stated reason.
    Refused(String),
}

pub type SkillResult = Result<Value, Exit>;

fn digest_bytes(b: &[u8]) -> String {
    format!("sha256:{}", hex::encode(Sha256::digest(b)))
}

fn compact(v: Option<&Value>) -> String {
    match v {
        None => String::new(),
        Some(val) => serde_json::to_string(val).expect("value serializes"),
    }
}

/// sha256 over kind\0id\0compact(payload)\0compact(schema) — the IR digest.
fn request_digest(r: &Request) -> String {
    let mut buf: Vec<u8> = Vec::new();
    buf.extend_from_slice(r.kind.as_bytes());
    buf.push(0);
    buf.extend_from_slice(r.id.as_bytes());
    buf.push(0);
    buf.extend_from_slice(compact(Some(&r.payload)).as_bytes());
    buf.push(0);
    buf.extend_from_slice(compact(r.output_schema.as_ref()).as_bytes());
    digest_bytes(&buf)
}

fn emit(output: ProgramOutput) -> ! {
    let mut out = std::io::stdout();
    let line = serde_json::to_string(&output).expect("output serializes");
    writeln!(out, "{}", line).expect("stdout writable");
    out.flush().ok();
    exit(0);
}

/// The replay cursor and the five primitives.
pub struct Context {
    journal: Journal,
    idx: usize,
    requirements: Vec<Requirement>,
}

impl Context {
    /// Yield a question asked through the host's normal interface.
    /// `options` are `(value, label)` pairs; pass `&[]` for a free answer.
    pub fn ask_user(&mut self, id: &str, question: &str, options: &[(&str, &str)]) -> String {
        let mut payload = json!({ "question": question });
        if !options.is_empty() {
            payload["options"] = Value::Array(
                options
                    .iter()
                    .map(|(v, l)| json!({ "value": v, "label": l }))
                    .collect(),
            );
        }
        let resp = self.step(Request {
            id: id.to_string(),
            kind: "ask_user".to_string(),
            payload,
            output_schema: Some(json!({
                "type": "object",
                "required": ["value"],
                "properties": { "value": { "type": "string" } }
            })),
        });
        resp.result["value"].as_str().unwrap_or_default().to_string()
    }

    /// Delegate reasoning to the model. `schema` (JSON Schema) is enforced
    /// by the supervisor on resume; the returned value is schema-valid by
    /// construction.
    pub fn agent_task(
        &mut self,
        id: &str,
        instruction: &str,
        context: Option<Value>,
        schema: Option<Value>,
    ) -> Value {
        let mut payload = json!({ "instruction": instruction });
        if let Some(c) = context {
            payload["context"] = c;
        }
        self.step(Request {
            id: id.to_string(),
            kind: "agent_task".to_string(),
            payload,
            output_schema: schema,
        })
        .result
    }

    /// Yield a command that yskill executes itself — the result is observed
    /// fact, not the agent's account of it.
    pub fn run_command(&mut self, id: &str, command: &str, timeout_seconds: u64) -> CommandResult {
        let mut payload = json!({ "command": command });
        if timeout_seconds > 0 {
            payload["timeout_seconds"] = json!(timeout_seconds);
        }
        let resp = self.step(Request {
            id: id.to_string(),
            kind: "run_command".to_string(),
            payload,
            output_schema: None,
        });
        serde_json::from_value(resp.result).unwrap_or(CommandResult {
            exit_code: -1,
            stdout: String::new(),
            stderr: String::new(),
            timed_out: false,
        })
    }

    /// Bind a claim to evidence. A failed requirement terminates the
    /// program immediately; completion is structurally unreachable past it.
    pub fn require(&mut self, ok: bool, claim: &str, evidence: Option<&Value>) {
        let mut req = Requirement {
            claim: claim.to_string(),
            passed: ok,
            evidence_digest: None,
        };
        if let Some(e) = evidence {
            req.evidence_digest = Some(digest_bytes(compact(Some(e)).as_bytes()));
        }
        self.requirements.push(req);
        if !ok {
            emit(ProgramOutput {
                kind: "terminal".to_string(),
                envelope: None,
                terminal: Some(TerminalOutcome {
                    status: "requirement_failed".to_string(),
                    result: None,
                    reason: Some(claim.to_string()),
                }),
                divergence: None,
                requirements: std::mem::take(&mut self.requirements),
            });
        }
    }

    pub fn blocked(&self, reason: &str) -> Exit {
        Exit::Blocked(reason.to_string())
    }

    pub fn refused(&self, reason: &str) -> Exit {
        Exit::Refused(reason.to_string())
    }

    /// The certified contract's step: replay with a mandatory per-step
    /// digest check, or emit-and-exit at the frontier. Consuming a recorded
    /// response for a drifted operation is the forbidden state the rival
    /// design fails (docs/locus/drv-ad50b13e…).
    fn step(&mut self, req: Request) -> ResponseEnvelope {
        let seq = (self.idx + 1) as u64;
        if self.idx < self.journal.entries.len() {
            let entry = &self.journal.entries[self.idx];
            let want = request_digest(&entry.request);
            let got = request_digest(&req);
            if want != got {
                emit(ProgramOutput {
                    kind: "diverged".to_string(),
                    envelope: None,
                    terminal: None,
                    divergence: Some(Divergence {
                        sequence: seq,
                        expected_digest: want,
                        got_digest: got,
                        detail: format!(
                            "replay produced operation \"{}\" ({}) where the journal recorded \"{}\" ({})",
                            req.id, req.kind, entry.request.id, entry.request.kind
                        ),
                    }),
                    requirements: Vec::new(),
                });
            }
            self.idx += 1;
            return self.journal.entries[self.idx - 1].response.clone();
        }
        self.idx += 1;
        emit(ProgramOutput {
            kind: "request".to_string(),
            envelope: Some(RequestEnvelope {
                protocol: PROTOCOL.to_string(),
                run_id: self.journal.run_id.clone(),
                skill: self.journal.skill.clone(),
                sequence: seq,
                request: req,
            }),
            terminal: None,
            divergence: None,
            requirements: self.requirements.clone(),
        });
    }
}

/// Run a skill program under the supervisor protocol. The program's return
/// value is the run result; return `Err(ctx.blocked(...))` or
/// `Err(ctx.refused(...))` for the honest terminals.
pub fn define_skill(program: fn(&mut Context) -> SkillResult) -> ! {
    let path = match std::env::var("YIELD_JOURNAL") {
        Ok(p) => p,
        Err(_) => {
            eprintln!("yield: YIELD_JOURNAL is not set; this program is run by yskill, not directly");
            exit(2);
        }
    };
    let bytes = match std::fs::read(&path) {
        Ok(b) => b,
        Err(err) => {
            eprintln!("yield: cannot read journal: {err}");
            exit(2);
        }
    };
    let journal: Journal = match serde_json::from_slice(&bytes) {
        Ok(j) => j,
        Err(err) => {
            eprintln!("yield: corrupt journal: {err}");
            exit(2);
        }
    };
    let mut ctx = Context {
        journal,
        idx: 0,
        requirements: Vec::new(),
    };
    match program(&mut ctx) {
        Ok(result) => emit(ProgramOutput {
            kind: "terminal".to_string(),
            envelope: None,
            terminal: Some(TerminalOutcome {
                status: "completed".to_string(),
                result: Some(result),
                reason: None,
            }),
            divergence: None,
            requirements: ctx.requirements,
        }),
        Err(Exit::Blocked(reason)) => emit(ProgramOutput {
            kind: "terminal".to_string(),
            envelope: None,
            terminal: Some(TerminalOutcome {
                status: "blocked".to_string(),
                result: None,
                reason: Some(reason),
            }),
            divergence: None,
            requirements: ctx.requirements,
        }),
        Err(Exit::Refused(reason)) => emit(ProgramOutput {
            kind: "terminal".to_string(),
            envelope: None,
            terminal: Some(TerminalOutcome {
                status: "refused".to_string(),
                result: None,
                reason: Some(reason),
            }),
            divergence: None,
            requirements: ctx.requirements,
        }),
    }
}
