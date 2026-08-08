# Repository formatting

Run this command before you open a pull request:

```bash
npm run format
```

Run this command to check formatting without changing files:

```bash
npm run format:check
```

Yield uses the standard formatter for each source type:

| Source                                           | Formatter                                                          |
| ------------------------------------------------ | ------------------------------------------------------------------ |
| JavaScript, TypeScript, JSON, Markdown, and YAML | [Prettier](https://prettier.io/docs/install.html)                  |
| Python                                           | [Ruff](https://docs.astral.sh/ruff/formatter/)                     |
| Go                                               | [gofmt](https://pkg.go.dev/cmd/gofmt)                              |
| Rust                                             | [rustfmt](https://doc.rust-lang.org/cargo/commands/cargo-fmt.html) |
| Shell                                            | [shfmt](https://github.com/mvdan/sh)                               |
| TOML                                             | [Taplo](https://taplo.tamasfe.dev/cli/introduction.html)           |

The command pins third-party formatter versions. Go and Rust use the repository
toolchain versions. GitHub Actions runs the check and does not rewrite files.

The formatter skips generated files. It also skips evaluation sources whose
exact bytes belong to a committed receipt. Run the relevant generator or
evaluation when you change those sources.
