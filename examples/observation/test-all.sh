#!/usr/bin/env bash
set -euo pipefail

example_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$example_dir/../.." && pwd)"
fixture="$repo_root/ir/yield.observation.v1/testdata/run-receipt.canonical.jsonl"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

canonical="$work_dir/receipt.json"
trailing="$work_dir/receipt-trailing.json"
mismatch="$work_dir/receipt-mismatch.json"
unknown="$work_dir/receipt-unknown.json"
secret="DO_NOT_PRINT_OBSERVATION_SECRET"

node --input-type=module - "$fixture" "$canonical" "$trailing" "$mismatch" "$unknown" "$secret" <<'NODE'
import { readFileSync, writeFileSync } from "node:fs"

const [, , fixture, canonicalPath, trailingPath, mismatchPath, unknownPath, secret] =
  process.argv
const framed = readFileSync(fixture)
if (framed.length < 2 || framed.at(-1) !== 0x0a || framed.subarray(0, -1).includes(0x0a)) {
  throw new Error("shared receipt fixture must contain one JSON record and one LF")
}

const canonical = framed.subarray(0, -1)
const document = JSON.parse(canonical.toString("utf8"))
writeFileSync(canonicalPath, canonical)
writeFileSync(trailingPath, Buffer.concat([canonical, Buffer.from("\n")]))
writeFileSync(
  mismatchPath,
  canonical.toString("utf8").replace(document.receipt_digest, `sha256:${"0".repeat(64)}`),
)
document.prompt = secret
writeFileSync(unknownPath, JSON.stringify(document))
NODE

run_example() {
	local language="$1"
	local receipt="$2"
	case "$language" in
	go)
		(cd "$repo_root" && go run ./examples/observation/go "$receipt")
		;;
	typescript)
		node "$example_dir/typescript/main.ts" "$receipt"
		;;
	python)
		PYTHONDONTWRITEBYTECODE=1 python3 "$example_dir/python/main.py" "$receipt"
		;;
	rust)
		cargo run --quiet --manifest-path "$example_dir/rust/Cargo.toml" -- "$receipt"
		;;
	*)
		echo "unknown language: $language" >&2
		return 1
		;;
	esac
}

expected='{"operation_summaries":[{"completed":1,"kind":"agent_task","requested":1,"total_elapsed_ms":3000}],"phase":"terminal","receipt_digest":"sha256:a0995d74d51e472f61dc2001f82b93fd8086f9c2f2816a22bb7e5c7fa3c01cdd","run_id":"run_fixture","schema":"yield.observation.v1","skill":"fixture","terminal_disposition":"completed"}'

for language in go typescript python rust; do
	output="$(run_example "$language" "$canonical")"
	normalized="$(jq -ceS . <<<"$output")"
	if [[ "$normalized" != "$expected" ]]; then
		echo "$language emitted an unexpected receipt summary" >&2
		exit 1
	fi

	for invalid in "$trailing" "$mismatch" "$unknown"; do
		stderr="$work_dir/${language}-$(basename "$invalid").stderr"
		if output="$(run_example "$language" "$invalid" 2>"$stderr")"; then
			echo "$language accepted invalid receipt bytes" >&2
			exit 1
		fi
		if [[ -n "$output" ]]; then
			echo "$language emitted a partial summary for invalid receipt bytes" >&2
			exit 1
		fi
		if grep -Fq "$secret" "$stderr"; then
			echo "$language exposed a private unknown-field value" >&2
			exit 1
		fi
	done

	echo "validated observation reader: $language"
done
