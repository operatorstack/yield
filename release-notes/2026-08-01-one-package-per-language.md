# One package per language

- Install one TypeScript, Python, Go, or Rust package to get both the SDK and
  the matching `yskill` runtime.
- Add `yskill --version`, `yskill version`, and language-aware `yskill init`
  scaffolds pinned to the installed version.
- Carry immutable Go runtimes for macOS, Linux, and Windows on amd64 and arm64.
- Fail clearly on unsupported or incomplete installations. The wrappers never
  download another runtime or search `PATH`.
- Verify the package launchers, runtime checksums, and shared IR behavior before
  publishing the public packages.
