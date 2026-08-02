"""Yield skill-program SDK for Python (yield.v1).

Implements the Locus-certified SDK execution contract (see ir/README.md):
load the journal, replay recorded operations with a digest comparison at
EVERY replayed step before consuming its response, emit exactly one
program output (request | terminal | diverged) on stdout, then exit.

Programs must be deterministic between yields: same journal, same
operations, every execution. Clocks, RNGs, environment reads, and
filesystem state are side effects — cross them through a yielded
operation or leave them out.
"""

from __future__ import annotations

import hashlib
import json
import os
import sys
from dataclasses import dataclass
from typing import Any, Callable, Optional

__all__ = [
    "Blocked",
    "Refused",
    "CommandResult",
    "Context",
    "define_skill",
]

_PROTOCOL = "yield.v1"


class Blocked(Exception):
    """Terminal exit: a true frontier was reached — say so explicitly."""

    def __init__(self, reason: str):
        super().__init__(f"blocked: {reason}")
        self.reason = reason


class Refused(Exception):
    """Terminal exit: the skill declines to proceed, with a stated reason."""

    def __init__(self, reason: str):
        super().__init__(f"refused: {reason}")
        self.reason = reason


class _EmitSignal(Exception):
    """Internal: unwinds the program once an output has been decided."""

    def __init__(self, output: dict):
        super().__init__("yield emit")
        self.output = output


@dataclass
class CommandResult:
    exit_code: int
    stdout: str
    stderr: str
    timed_out: bool = False


def _digest(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def _compact(value: Any) -> str:
    if value is None:
        return ""
    return json.dumps(value, separators=(",", ":"))


def _request_digest(req: dict) -> str:
    """sha256 over kind\\0id\\0compact(payload)\\0compact(schema) — the IR digest."""
    parts = b"\x00".join(
        [
            str(req.get("kind", "")).encode(),
            str(req.get("id", "")).encode(),
            _compact(req.get("payload")).encode(),
            _compact(req.get("output_schema")).encode(),
        ]
    )
    return _digest(parts)


class Context:
    """The replay cursor and the five primitives."""

    def __init__(self, journal: dict):
        self._journal = journal
        self._entries = journal.get("entries") or []
        self._idx = 0
        self._requirements: list[dict] = []

    # -- primitives --------------------------------------------------------

    def ask_user(self, id: str, question: str, options: Optional[list] = None) -> str:
        payload: dict = {"question": question}
        value_schema: dict = {"type": "string"}
        if options:
            payload["options"] = options
            value_schema["enum"] = [option["value"] for option in options]
        resp = self._step(
            {
                "id": id,
                "kind": "ask_user",
                "payload": payload,
                "output_schema": {
                    "type": "object",
                    "required": ["value"],
                    "additionalProperties": False,
                    "properties": {"value": value_schema},
                },
            }
        )
        return resp["result"]["value"]

    def agent_task(
        self,
        id: str,
        instruction: str,
        context: Any = None,
        schema: Any = None,
    ) -> Any:
        """Delegate reasoning to the model; the result is schema-valid by
        construction (the supervisor enforces the schema on resume)."""
        payload: dict = {"instruction": instruction}
        if context is not None:
            payload["context"] = context
        req = {"id": id, "kind": "agent_task", "payload": payload}
        if schema is not None:
            req["output_schema"] = schema
        return self._step(req)["result"]

    def run_command(self, id: str, command: str, timeout_seconds: int = 0) -> CommandResult:
        """Yield a command that yskill executes itself — the result is
        observed fact, not the agent's account of it."""
        payload: dict = {"command": command}
        if timeout_seconds > 0:
            payload["timeout_seconds"] = timeout_seconds
        result = self._step({"id": id, "kind": "run_command", "payload": payload})["result"]
        return CommandResult(
            exit_code=result.get("exit_code", 0),
            stdout=result.get("stdout", ""),
            stderr=result.get("stderr", ""),
            timed_out=result.get("timed_out", False),
        )

    def require(self, ok: bool, claim: str, evidence: Any = None) -> None:
        """Bind a claim to evidence. A failed requirement terminates the
        program immediately; completion is structurally unreachable past it."""
        req = {"claim": claim, "passed": bool(ok)}
        if evidence is not None:
            if isinstance(evidence, CommandResult):
                evidence = evidence.__dict__
            req["evidence_digest"] = _digest(_compact(evidence).encode())
        self._requirements.append(req)
        if not ok:
            raise _EmitSignal(
                {
                    "type": "terminal",
                    "terminal": {"status": "requirement_failed", "reason": claim},
                    "requirements": self._requirements,
                }
            )

    def blocked(self, reason: str) -> None:
        raise Blocked(reason)

    def refused(self, reason: str) -> None:
        raise Refused(reason)

    # -- the certified contract's step --------------------------------------

    def _step(self, req: dict) -> dict:
        seq = self._idx + 1
        if self._idx < len(self._entries):
            entry = self._entries[self._idx]
            want = _request_digest(entry["request"])
            got = _request_digest(req)
            if want != got:
                # Mandatory per-step check: consuming a recorded response
                # for a drifted operation is the forbidden state the rival
                # design fails (docs/locus/drv-ad50b13e…).
                raise _EmitSignal(
                    {
                        "type": "diverged",
                        "divergence": {
                            "sequence": seq,
                            "expected_digest": want,
                            "got_digest": got,
                            "detail": (
                                f'replay produced operation "{req["id"]}" ({req["kind"]}) '
                                f'where the journal recorded "{entry["request"]["id"]}" '
                                f'({entry["request"]["kind"]})'
                            ),
                        },
                    }
                )
            self._idx += 1
            return entry["response"]
        self._idx += 1
        raise _EmitSignal(
            {
                "type": "request",
                "envelope": {
                    "protocol": _PROTOCOL,
                    "run_id": self._journal["run_id"],
                    "skill": self._journal["skill"],
                    "sequence": seq,
                    "request": req,
                },
                "requirements": self._requirements,
            }
        )

    # -- terminals -----------------------------------------------------------

    def _terminal_for(self, err: BaseException) -> Optional[dict]:
        if isinstance(err, _EmitSignal):
            return err.output
        if isinstance(err, Blocked):
            return {
                "type": "terminal",
                "terminal": {"status": "blocked", "reason": err.reason},
                "requirements": self._requirements,
            }
        if isinstance(err, Refused):
            return {
                "type": "terminal",
                "terminal": {"status": "refused", "reason": err.reason},
                "requirements": self._requirements,
            }
        return None

    def _completed(self, result: Any) -> dict:
        return {
            "type": "terminal",
            "terminal": {"status": "completed", "result": result},
            "requirements": self._requirements,
        }


def _emit(output: dict) -> None:
    sys.stdout.write(json.dumps(output, separators=(",", ":")) + "\n")
    sys.stdout.flush()
    sys.exit(0)


def define_skill(program: Callable[[Context], Any]) -> None:
    """Run a skill program under the supervisor protocol. The program's
    return value is the run result; call ctx.blocked()/ctx.refused() for
    the honest terminals."""
    path = os.environ.get("YIELD_JOURNAL")
    if not path:
        print(
            "yield: YIELD_JOURNAL is not set; this program is run by yskill, not directly",
            file=sys.stderr,
        )
        sys.exit(2)
    try:
        with open(path, "r", encoding="utf-8") as f:
            journal = json.load(f)
    except (OSError, json.JSONDecodeError) as err:
        print(f"yield: cannot read journal: {err}", file=sys.stderr)
        sys.exit(2)
    ctx = Context(journal)
    try:
        result = program(ctx)
    except (Blocked, Refused, _EmitSignal) as err:
        _emit(ctx._terminal_for(err))
        return
    _emit(ctx._completed(result))
