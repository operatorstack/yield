// QA a web change. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command(
        "build-web",
        "printf 'build passed; changed routes: / and /settings\\n'",
        300,
    );
    ctx.require(
        preflight.exit_code == 0,
        "the web application builds",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "test-changed-routes",
        "Test the changed routes at desktop and mobile sizes, including keyboard navigation and form errors. Return pass only when no blocking regression remains.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the changed routes have no blocking regression",
        Some(&decision),
    );

    Ok(json!({"workflow": "qa-web-change", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
