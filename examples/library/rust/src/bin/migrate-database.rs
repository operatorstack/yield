// Run a database migration. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command(
        "dry-run-migration",
        "printf 'dry run: add users_email_idx concurrently\\n'",
        300,
    );
    ctx.require(
        preflight.exit_code == 0,
        "the migration dry-run succeeds",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "review-migration",
        "Review the migration plan for lock risk, irreversible work, and rollback. Return pass only when the plan is safe to apply.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the migration plan has acceptable risk",
        Some(&decision),
    );

    let approval = ctx.ask_user(
        "approve-migration",
        "Apply the reviewed database migration?",
        &[("continue", "Continue"), ("stop", "Stop")],
    );
    if approval != "continue" {
        return Err(ctx.refused("the operator declined to continue"));
    }

    let action = ctx.run_command("apply-migration", "printf 'migration applied\\n'", 600);
    ctx.require(
        action.exit_code == 0,
        "the migration applies cleanly",
        Some(&json!({"exit_code": action.exit_code})),
    );

    let verify = ctx.run_command(
        "verify-migration",
        "printf 'migration verification passed\\n'",
        300,
    );
    ctx.require(
        verify.exit_code == 0,
        "the migrated database passes verification",
        Some(&json!({"exit_code": verify.exit_code})),
    );

    Ok(json!({"workflow": "migrate-database", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
