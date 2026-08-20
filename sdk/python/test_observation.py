import json
import unittest
from pathlib import Path

from yieldskill import ReceiptError, parse_run_receipt, verify_run_receipt


FIXTURE = (
    Path(__file__).parents[2]
    / "ir"
    / "yield.observation.v1"
    / "testdata"
    / "run-receipt.canonical.jsonl"
).read_bytes()[:-1]


class ReceiptTests(unittest.TestCase):
    def test_shared_go_canonical_receipt(self):
        receipt = parse_run_receipt(FIXTURE)
        self.assertEqual(
            receipt["receipt_digest"],
            "sha256:a0995d74d51e472f61dc2001f82b93fd8086f9c2f2816a22bb7e5c7fa3c01cdd",
        )
        self.assertEqual(receipt["operations"][0]["kind"], "agent_task")
        verify_run_receipt(receipt, FIXTURE)

    def test_rejects_unknown_noncanonical_and_digest_mismatch(self):
        document = json.loads(FIXTURE)
        document["prompt"] = "secret"
        with self.assertRaises(ReceiptError):
            parse_run_receipt(json.dumps(document, separators=(",", ":")).encode())
        with self.assertRaisesRegex(ReceiptError, "canonical"):
            parse_run_receipt(FIXTURE + b"\n")
        with self.assertRaisesRegex(ReceiptError, "digest"):
            parse_run_receipt(
                FIXTURE.replace(
                    document["receipt_digest"].encode(),
                    ("sha256:" + "0" * 64).encode(),
                )
            )


if __name__ == "__main__":
    unittest.main()
