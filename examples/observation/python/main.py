import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "sdk" / "python"))

from yieldskill import parse_run_receipt  # noqa: E402


def main() -> int:
    if len(sys.argv) != 2:
        print(
            "usage: python3 examples/observation/python/main.py <canonical-receipt.json>",
            file=sys.stderr,
        )
        return 1

    try:
        receipt = parse_run_receipt(Path(sys.argv[1]).read_bytes())
    except (OSError, ValueError) as error:
        print(f"receipt example: {error}", file=sys.stderr)
        return 1

    summary = {
        "schema": receipt["schema"],
        "receipt_digest": receipt["receipt_digest"],
        "run_id": receipt["run"]["id"],
        "skill": receipt["skill"]["name"],
        "phase": receipt["outcome"]["phase"],
        "terminal_disposition": receipt["outcome"].get("terminal_disposition"),
        "operation_summaries": receipt["operation_summaries"],
    }
    print(json.dumps(summary, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
