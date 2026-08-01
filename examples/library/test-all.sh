#!/usr/bin/env bash
set -euo pipefail

library_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
yskill="${YSKILL:-yskill}"
count=0

for language in typescript python go rust; do
  for skill_dir in "$library_dir/$language"/*; do
    [[ -f "$skill_dir/fixtures/responses.json" ]] || continue
    echo "==> ${language}/$(basename "$skill_dir")"
    "$yskill" test "$skill_dir"
    count=$((count + 1))
  done
done

if [[ "$count" -ne 40 ]]; then
  echo "expected 40 fixture runs, found $count" >&2
  exit 1
fi

echo "validated $count example workflows"
