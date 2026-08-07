// Example skill (Rust): a database migration with the discipline in code —
// dry-run, show the diff, human approval, apply, verify. The append-only
// run log is the audit trail; every irreversible action has a recorded,
// approved request before it.
// README_EXAMPLE_START
use serde_json::json;
use yieldskill::{define_skill, Context, SkillResult};

fn program(ctx: &mut Context) -> SkillResult {
    let dry_run = ctx.run_command("dry-run", "echo 'plan: add index users_email_idx'", 300);
    ctx.require(
        dry_run.exit_code == 0,
        "the dry run succeeds",
        Some(&json!({ "exit_code": dry_run.exit_code })),
    );

    let review = ctx.agent_task(
        "summarize-plan",
        "Summarize the migration plan for the operator: what changes, what is irreversible, what the rollback is.",
        Some(json!({ "plan": dry_run.stdout })),
        Some(json!({
            "type": "object",
            "required": ["summary"],
            "properties": { "summary": { "type": "string", "minLength": 1 } }
        })),
    );

    let approval = ctx.ask_user(
        "approve-apply",
        "Apply the migration to the live database?",
        &[("apply", "Apply now"), ("abort", "Abort")],
    );
    if approval != "apply" {
        return Err(ctx.refused("the operator declined to apply the migration"));
    }

    let apply = ctx.run_command("apply", "echo apply-ok", 600);
    ctx.require(
        apply.exit_code == 0,
        "the migration applies cleanly",
        Some(&json!({ "exit_code": apply.exit_code })),
    );

    let verify = ctx.run_command("verify", "echo verify-ok", 300);
    ctx.require(
        verify.exit_code == 0,
        "post-apply verification passes",
        Some(&json!({ "exit_code": verify.exit_code })),
    );

    Ok(json!({
        "applied": true,
        "summary": review["summary"],
    }))
}

fn main() {
    define_skill(program);
}
// README_EXAMPLE_END
