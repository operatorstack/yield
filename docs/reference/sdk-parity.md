# Four SDKs, one protocol

Go, TypeScript, Python, and Rust implement the same observable `yield.v1`
contract.

| Language   | Package                                    | Program entry                       |
| ---------- | ------------------------------------------ | ----------------------------------- |
| TypeScript | `@operatorstack/yield`                     | `defineSkill((ctx) => value)`       |
| Python     | `yieldskill`                               | `define_skill(program)`             |
| Go         | `github.com/operatorstack/yield/sdk/yield` | `yield.Main(program)`               |
| Rust       | `yieldskill`                               | `yieldskill::define_skill(program)` |

Skills declare their language and runner in `skill.json`, for example:

```json
{ "version": 1, "language": "typescript", "run": ["node", "main.ts"] }
```

The conformance suite runs the same workflow in all four languages and compares
the observable protocol traces. Language-specific types and syntax differ; run
IDs, operation sequencing, digests, responses, requirements, divergence, and
terminal outcomes do not.

Use the language already present in the repository. A mixed-language skill is
usually harder to install and maintain without changing what Yield can express.
