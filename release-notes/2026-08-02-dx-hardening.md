# Safer responses and simpler workflow setup

- Add `yskill respond` so agents can answer a pending step without building a
  response file by hand.
- Serialize responses per run across processes, recover exact retries, and
  refuse conflicting content.
- Enforce declared `AskUser` options in the TypeScript, Python, Go, and Rust
  SDKs.
- Let `yskill doctor` check a workflow without requiring agent adapters, and
  keep fixture test runs temporary by default.
- Add bulk adapter registration, safe synchronization, terminal-run pruning,
  relocatable Python workflows, and deterministic fixture effects.
- Verify 40 workflow cases and eight runtime checks, including concurrent
  response admission and recovery.
