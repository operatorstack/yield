// Release a package. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command("test-package", "printf 'package tests passed\\n'", 300);
    ctx.require(
        preflight.exit_code == 0,
        "the package tests pass",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "review-release",
        "Review the pending package release for breaking changes, missing notes, and rollback risk. Return pass only when it is ready to publish.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the package is ready to publish",
        Some(&decision),
    );

    let approval = ctx.ask_user(
        "approve-publish",
        "Publish this package release?",
        &[("continue", "Continue"), ("stop", "Stop")],
    );
    if approval != "continue" {
        return Err(ctx.refused("the operator declined to continue"));
    }

    let action = ctx.run_command("publish-package", "printf 'package published\\n'", 600);
    ctx.require(
        action.exit_code == 0,
        "the package publish command succeeds",
        Some(&json!({"exit_code": action.exit_code})),
    );

    let verify = ctx.run_command(
        "verify-package",
        "printf 'published package resolved from registry\\n'",
        300,
    );
    ctx.require(
        verify.exit_code == 0,
        "the published package resolves from the registry",
        Some(&json!({"exit_code": verify.exit_code})),
    );

    Ok(json!({"workflow": "release-package", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
