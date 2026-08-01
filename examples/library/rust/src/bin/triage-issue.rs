// Triage an issue. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command(
        "read-issue",
        "printf 'issue: intermittent timeout after retry change\\n'",
        300,
    );
    ctx.require(
        preflight.exit_code == 0,
        "the issue report is available",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "classify-issue",
        "Classify severity, identify missing evidence, and propose exactly one next action. Return pass only when the summary is actionable.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the issue has one actionable next step",
        Some(&decision),
    );

    Ok(json!({"workflow": "triage-issue", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
