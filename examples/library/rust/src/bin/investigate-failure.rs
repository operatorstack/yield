// Investigate a failure. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command(
        "capture-failure",
        "printf 'failing test captured with recent diff\\n'",
        300,
    );
    ctx.require(
        preflight.exit_code == 0,
        "the failure evidence is captured",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "diagnose-cause",
        "Use the failure output and recent change to identify the most likely root cause. Return pass only when the summary states a causal chain.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the diagnosis states a supported cause",
        Some(&decision),
    );

    Ok(json!({"workflow": "investigate-failure", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
