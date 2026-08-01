// Audit a change for security. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command(
        "run-security-checks",
        "printf 'dependency and secret scans completed\\n'",
        300,
    );
    ctx.require(
        preflight.exit_code == 0,
        "the mechanical security checks complete",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "review-trust-boundaries",
        "Review authentication, authorization, input handling, secrets, and trust-boundary changes. Return pass only when no critical risk remains.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the change has no critical security finding",
        Some(&decision),
    );

    Ok(json!({"workflow": "audit-security", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
