# Fresh Go scaffolds run immediately

- Resolve the pinned Yield module when `yskill init` creates a Go workflow.
- Keep later runs read-only so dependency files cannot change after the run is
  bound to its source digest.
- Verify a new scaffold completes `yskill test` without a pre-existing
  `go.sum` file.
