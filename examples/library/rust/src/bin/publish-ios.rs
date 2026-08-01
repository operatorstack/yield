// Publish an iOS build. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command(
        "archive-ios",
        "printf 'iOS archive and tests passed\\n'",
        300,
    );
    ctx.require(
        preflight.exit_code == 0,
        "the iOS archive and tests pass",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "review-ios-release",
        "Review the iOS release metadata, versioning, privacy notes, and rollout risk. Return pass only when the build is ready for upload.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the iOS build is ready for upload",
        Some(&decision),
    );

    let approval = ctx.ask_user(
        "approve-ios-upload",
        "Upload this iOS build to App Store Connect?",
        &[("continue", "Continue"), ("stop", "Stop")],
    );
    if approval != "continue" {
        return Err(ctx.refused("the operator declined to continue"));
    }

    let action = ctx.run_command("upload-ios", "printf 'iOS build uploaded\\n'", 600);
    ctx.require(
        action.exit_code == 0,
        "the iOS upload command succeeds",
        Some(&json!({"exit_code": action.exit_code})),
    );

    let verify = ctx.run_command(
        "verify-ios-processing",
        "printf 'uploaded build entered processing\\n'",
        300,
    );
    ctx.require(
        verify.exit_code == 0,
        "the uploaded iOS build entered processing",
        Some(&json!({"exit_code": verify.exit_code})),
    );

    Ok(json!({"workflow": "publish-ios", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
