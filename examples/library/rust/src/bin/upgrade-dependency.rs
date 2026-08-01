// Upgrade a dependency. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command("baseline-tests", "printf 'baseline tests passed\\n'", 300);
    ctx.require(
        preflight.exit_code == 0,
        "the baseline tests pass",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "review-upgrade",
        "Review the dependency upgrade for API changes, migration work, and rollback risk. Return pass only when the change is bounded.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the dependency upgrade has a bounded plan",
        Some(&decision),
    );

    let approval = ctx.ask_user(
        "approve-upgrade",
        "Apply the reviewed dependency upgrade?",
        &[("continue", "Continue"), ("stop", "Stop")],
    );
    if approval != "continue" {
        return Err(ctx.refused("the operator declined to continue"));
    }

    let action = ctx.run_command("apply-upgrade", "printf 'dependency upgraded\\n'", 600);
    ctx.require(
        action.exit_code == 0,
        "the dependency upgrade command succeeds",
        Some(&json!({"exit_code": action.exit_code})),
    );

    let verify = ctx.run_command(
        "post-upgrade-tests",
        "printf 'post-upgrade tests passed\\n'",
        300,
    );
    ctx.require(
        verify.exit_code == 0,
        "the tests pass after the dependency upgrade",
        Some(&json!({"exit_code": verify.exit_code})),
    );

    Ok(json!({"workflow": "upgrade-dependency", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
