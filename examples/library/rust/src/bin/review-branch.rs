// Review a branch. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command(
        "check-branch",
        "printf 'typecheck and tests passed\\n'",
        300,
    );
    ctx.require(
        preflight.exit_code == 0,
        "the branch passes mechanical checks",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "review-diff",
        "Review the branch for correctness, security, data-loss risks, and missing tests. Return pass only when no critical finding remains.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the review has no critical findings",
        Some(&decision),
    );

    Ok(json!({"workflow": "review-branch", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
