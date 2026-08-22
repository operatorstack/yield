use serde::Serialize;
use std::env;
use std::fs;
use std::io::{self, Write};
use std::process::ExitCode;
use yieldskill::observation::{OperationSummary, RunReceipt, TerminalDisposition};

#[derive(Serialize)]
struct ReceiptSummary<'a> {
    schema: &'a str,
    receipt_digest: &'a str,
    run_id: &'a str,
    skill: &'a str,
    phase: &'a yieldskill::observation::LifecyclePhase,
    terminal_disposition: &'a Option<TerminalDisposition>,
    operation_summaries: &'a [OperationSummary],
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    let arguments: Vec<String> = env::args().collect();
    if arguments.len() != 2 {
        return Err(
            "usage: cargo run --manifest-path examples/observation/rust/Cargo.toml -- <canonical-receipt.json>"
                .into(),
        );
    }

    let canonical = fs::read(&arguments[1])?;
    let receipt = RunReceipt::parse_and_verify(&canonical)?;
    let summary = ReceiptSummary {
        schema: &receipt.schema,
        receipt_digest: &receipt.receipt_digest,
        run_id: &receipt.run.id,
        skill: &receipt.skill.name,
        phase: &receipt.outcome.phase,
        terminal_disposition: &receipt.outcome.terminal_disposition,
        operation_summaries: &receipt.operation_summaries,
    };

    serde_json::to_writer(io::stdout().lock(), &summary)?;
    io::stdout().write_all(b"\n")?;
    Ok(())
}

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("receipt example: {error}");
            ExitCode::FAILURE
        }
    }
}
