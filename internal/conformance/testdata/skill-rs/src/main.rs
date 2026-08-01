// Conformance program (Rust). The SAME program exists in Go, TypeScript,
// and Python; the harness asserts identical observable protocol behavior.
use serde_json::json;
use yieldskill::{define_skill, Context, SkillResult};

fn program(ctx: &mut Context) -> SkillResult {
    let proceed = ctx.ask_user(
        "q1-proceed",
        "Proceed with the conformance run?",
        &[("yes", "Yes"), ("no", "No")],
    );
    if proceed == "no" {
        return Err(ctx.refused("operator declined"));
    }

    let t = ctx.agent_task(
        "t2-analyze",
        "Return {\"n\": <integer>}.",
        Some(json!({ "proceed": proceed })),
        Some(json!({
            "type": "object",
            "required": ["n"],
            "properties": { "n": { "type": "integer" } }
        })),
    );
    let n = t["n"].as_i64().unwrap_or(0);
    if n == 0 {
        return Err(ctx.blocked("n is zero: a true frontier"));
    }

    let c = ctx.run_command("c3-echo", "echo conform-ok", 0);
    ctx.require(n > 0, "n is positive", Some(&json!({ "n": n })));
    ctx.require(
        c.exit_code == 0,
        "the echo command passes",
        Some(&json!({ "exit_code": c.exit_code })),
    );

    Ok(json!({ "n": n }))
}

fn main() {
    define_skill(program);
}
