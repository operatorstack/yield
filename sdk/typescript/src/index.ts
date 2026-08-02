// Yield skill-program SDK for TypeScript (yield.v1).
//
// Implements the Locus-certified SDK execution contract (see ir/README.md):
// load the journal, replay recorded operations with a digest comparison at
// EVERY replayed step before consuming its response, emit exactly one
// program output (request | terminal | diverged) on stdout, then exit.
//
// Programs are synchronous and must be deterministic between yields: same
// journal, same operations, every execution. Wall clocks, RNGs, and
// filesystem reads are side effects — cross them through a yielded
// operation or leave them out.
//
// Runs under Node >= 23.6 (native type stripping): `node main.ts`.

import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { exit, env, stdout, stderr } from "node:process";

export type OpKind = "ask_user" | "agent_task" | "run_command";

export interface SkillRef {
  name: string;
  version?: string;
  digest: string;
}

export interface Request {
  id: string;
  kind: OpKind;
  payload: unknown;
  output_schema?: unknown;
}

export interface RequestEnvelope {
  protocol: "yield.v1";
  run_id: string;
  skill: SkillRef;
  sequence: number;
  request: Request;
}

export interface ResponseEnvelope {
  run_id: string;
  sequence: number;
  request_id: string;
  status: "completed" | "failed";
  result: unknown;
}

export interface Option {
  value: string;
  label?: string;
}

export interface CommandResult {
  exit_code: number;
  stdout: string;
  stderr: string;
  timed_out?: boolean;
}

export interface Requirement {
  claim: string;
  passed: boolean;
  evidence_digest?: string;
}

interface Journal {
  run_id: string;
  skill: SkillRef;
  entries?: { request: Request; response: ResponseEnvelope }[];
}

type ProgramOutput =
  | { type: "request"; envelope: RequestEnvelope; requirements?: Requirement[] }
  | {
      type: "terminal";
      terminal: {
        status: "completed" | "blocked" | "refused" | "requirement_failed";
        result?: unknown;
        reason?: string;
      };
      requirements?: Requirement[];
    }
  | {
      type: "diverged";
      divergence: {
        sequence: number;
        expected_digest: string;
        got_digest: string;
        detail?: string;
      };
      requirements?: Requirement[];
    };

/** Terminal exit: a true frontier was reached — say so explicitly. */
export class Blocked extends Error {
  reason: string;
  constructor(reason: string) {
    super(`blocked: ${reason}`);
    this.reason = reason;
  }
}

/** Terminal exit: the skill declines to proceed, with a stated reason. */
export class Refused extends Error {
  reason: string;
  constructor(reason: string) {
    super(`refused: ${reason}`);
    this.reason = reason;
  }
}

// Thrown to unwind the program when an output has been decided; defineSkill
// catches it. Never observable by program code that doesn't catch blindly.
class EmitSignal {
  output: ProgramOutput;
  constructor(output: ProgramOutput) {
    this.output = output;
  }
}

const digest = (b: string | Buffer): string =>
  "sha256:" + createHash("sha256").update(b).digest("hex");

const compact = (v: unknown): string =>
  v === undefined ? "" : JSON.stringify(v);

/** sha256 over kind\0id\0compact(payload)\0compact(schema) — the IR digest. */
const requestDigest = (r: Request): string =>
  digest(
    Buffer.concat([
      Buffer.from(String(r.kind)),
      Buffer.from([0]),
      Buffer.from(r.id),
      Buffer.from([0]),
      Buffer.from(compact(r.payload)),
      Buffer.from([0]),
      Buffer.from(compact(r.output_schema)),
    ]),
  );

export class Context {
  private idx = 0;
  private requirements: Requirement[] = [];
  private journal: Journal;

  constructor(journal: Journal) {
    this.journal = journal;
  }

  /** Yield a question asked through the host's normal interface. */
  askUser(id: string, question: string, options?: Option[]): string {
    const valueSchema: Record<string, unknown> = { type: "string" };
    if (options?.length) valueSchema.enum = options.map((option) => option.value);
    const resp = this.step({
      id,
      kind: "ask_user",
      payload: options ? { question, options } : { question },
      output_schema: {
        type: "object",
        required: ["value"],
        additionalProperties: false,
        properties: { value: valueSchema },
      },
    });
    return (resp.result as { value: string }).value;
  }

  /**
   * Delegate reasoning to the model. `schema` (JSON Schema) is enforced by
   * the supervisor on resume; the returned value is schema-valid by
   * construction.
   */
  agentTask<T = unknown>(
    id: string,
    instruction: string,
    context?: unknown,
    schema?: unknown,
  ): T {
    const resp = this.step({
      id,
      kind: "agent_task",
      payload: context === undefined ? { instruction } : { instruction, context },
      output_schema: schema,
    });
    return resp.result as T;
  }

  /**
   * Yield a command that yskill executes itself — the result is observed
   * fact, not the agent's account of it.
   */
  runCommand(id: string, command: string, timeoutSeconds = 0): CommandResult {
    const payload =
      timeoutSeconds > 0
        ? { command, timeout_seconds: timeoutSeconds }
        : { command };
    const resp = this.step({ id, kind: "run_command", payload });
    return resp.result as CommandResult;
  }

  /**
   * Bind a claim to evidence. A failed requirement terminates the program
   * immediately; completion is structurally unreachable past it.
   */
  require(ok: boolean, claim: string, evidence?: unknown): void {
    const req: Requirement = { claim, passed: ok };
    if (evidence !== undefined) req.evidence_digest = digest(compact(evidence));
    this.requirements.push(req);
    if (!ok) {
      throw new EmitSignal({
        type: "terminal",
        terminal: { status: "requirement_failed", reason: claim },
        requirements: this.requirements,
      });
    }
  }

  blocked(reason: string): never {
    throw new Blocked(reason);
  }

  refused(reason: string): never {
    throw new Refused(reason);
  }

  /** @internal replay-or-emit; the certified contract's step. */
  private step(req: Request): ResponseEnvelope {
    const entries = this.journal.entries ?? [];
    const seq = this.idx + 1;
    if (this.idx < entries.length) {
      const entry = entries[this.idx];
      const want = requestDigest(entry.request);
      const got = requestDigest(req);
      if (want !== got) {
        // Mandatory per-step check: consuming a recorded response for a
        // drifted operation is the forbidden state the rival design fails.
        throw new EmitSignal({
          type: "diverged",
          divergence: {
            sequence: seq,
            expected_digest: want,
            got_digest: got,
            detail: `replay produced operation "${req.id}" (${req.kind}) where the journal recorded "${entry.request.id}" (${entry.request.kind})`,
          },
        });
      }
      this.idx++;
      return entry.response;
    }
    this.idx++;
    throw new EmitSignal({
      type: "request",
      envelope: {
        protocol: "yield.v1",
        run_id: this.journal.run_id,
        skill: this.journal.skill,
        sequence: seq,
        request: req,
      },
      requirements: this.requirements,
    });
  }

  /** @internal */
  terminalFor(err: unknown): ProgramOutput | null {
    if (err instanceof EmitSignal) return err.output;
    if (err instanceof Blocked)
      return {
        type: "terminal",
        terminal: { status: "blocked", reason: err.reason },
        requirements: this.requirements,
      };
    if (err instanceof Refused)
      return {
        type: "terminal",
        terminal: { status: "refused", reason: err.reason },
        requirements: this.requirements,
      };
    return null;
  }

  /** @internal */
  completed(result: unknown): ProgramOutput {
    return {
      type: "terminal",
      terminal: { status: "completed", result },
      requirements: this.requirements,
    };
  }
}

function emit(output: ProgramOutput): never {
  stdout.write(JSON.stringify(output) + "\n");
  exit(0);
}

/**
 * Run a skill program under the supervisor protocol. The program's return
 * value is the run result; throw `ctx.blocked(...)`/`ctx.refused(...)` for
 * the honest terminals.
 */
export function defineSkill(program: (ctx: Context) => unknown): void {
  const path = env.YIELD_JOURNAL;
  if (!path) {
    stderr.write(
      "yield: YIELD_JOURNAL is not set; this program is run by yskill, not directly\n",
    );
    exit(2);
  }
  let journal: Journal;
  try {
    journal = JSON.parse(readFileSync(path, "utf8")) as Journal;
  } catch (err) {
    stderr.write(`yield: cannot read journal: ${String(err)}\n`);
    exit(2);
  }
  const ctx = new Context(journal!);
  try {
    emit(ctx.completed(program(ctx)));
  } catch (err) {
    const out = ctx.terminalFor(err);
    if (out) emit(out);
    stderr.write(`yield: program error: ${String(err)}\n`);
    exit(1);
  }
}
