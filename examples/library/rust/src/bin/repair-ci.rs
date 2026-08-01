// Repair a CI failure. Replace the illustrative commands with your project commands.
use serde_json::{json, Value};
use yieldskill::{define_skill, Context, SkillResult};

fn decision_schema() -> Value {
    json!({"type":"object","required":["status","critical","summary"],"properties":{"status":{"enum":["pass","needs_work"]},"critical":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1}}})
}

fn program(ctx: &mut Context) -> SkillResult {
    let preflight = ctx.run_command(
        "capture-ci-log",
        "printf 'ci log: test shard 2 failed after cache restore\\n'",
        300,
    );
    ctx.require(
        preflight.exit_code == 0,
        "the failing CI evidence is captured",
        Some(&json!({"exit_code": preflight.exit_code})),
    );

    let decision = ctx.agent_task(
        "plan-ci-repair",
        "Diagnose the CI failure and describe the smallest supported repair. Return pass only when the repair is tied to the observed log.",
        Some(json!({"stdout": preflight.stdout, "stderr": preflight.stderr})),
        Some(decision_schema()),
    );
    ctx.require(
        decision["status"] == "pass" && decision["critical"] == 0,
        "the CI repair is supported by the failure evidence",
        Some(&decision),
    );

    let action = ctx.run_command("apply-ci-repair", "printf 'ci repair applied\\n'", 600);
    ctx.require(
        action.exit_code == 0,
        "the CI repair command succeeds",
        Some(&json!({"exit_code": action.exit_code})),
    );

    let verify = ctx.run_command(
        "rerun-ci-check",
        "printf 'failing CI check now passes\\n'",
        300,
    );
    ctx.require(
        verify.exit_code == 0,
        "the previously failing CI check passes",
        Some(&json!({"exit_code": verify.exit_code})),
    );

    Ok(json!({"workflow": "repair-ci", "summary": decision["summary"]}))
}

fn main() {
    define_skill(program);
}
