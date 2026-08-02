# Accept generated Go SDK pins

- Let `doctor` read the single-line `go.mod` requirement produced by
  `yskill init`.
- Keep support for block-style Go requirements.
- Verify both forms against the repository-local Yield runtime version.
