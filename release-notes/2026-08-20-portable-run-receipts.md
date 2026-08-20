# Portable run receipts and deferred export

- Add the public `yield.observation.v1` RunReceipt schema and deterministic
  journal projection.
- Materialize privacy-safe, content-addressed receipts at foreground stopping
  points without changing replay or workflow results.
- Add explicit receipt inspection, deferred command-sink delivery, retry and
  status operations, and deterministic local reports.
- Record versioned source identity, runtime identity, typed initialization and
  execution failures, and optional experiment groups.
