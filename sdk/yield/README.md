<p align="center">
  <a href="https://yield.operatorstack.systems/">
    <img src="https://raw.githubusercontent.com/operatorstack/yield/main/assets/yield-mark.svg" width="96" alt="Yield" />
  </a>
</p>

<h1 align="center">Yield for Go</h1>

<p align="center"><strong>Move repeatable coding-agent instructions from words into Go.</strong></p>

<p align="center">
  Build typed, resumable workflows that stay beside the code they operate on.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/operatorstack/yield/sdk/yield"><img alt="Go reference" src="https://pkg.go.dev/badge/github.com/operatorstack/yield/sdk/yield.svg" /></a>
  <a href="https://github.com/operatorstack/yield/releases"><img alt="Go module version" src="https://img.shields.io/github/v/tag/operatorstack/yield?filter=v*&amp;style=flat-square&amp;label=module" /></a>
  <a href="https://github.com/operatorstack/yield/actions/workflows/verify.yml"><img alt="Build status" src="https://img.shields.io/github/actions/workflow/status/operatorstack/yield/verify.yml?branch=main&amp;style=flat-square&amp;label=build" /></a>
  <a href="https://github.com/operatorstack/yield/blob/main/LICENSE"><img alt="MIT license" src="https://img.shields.io/github/license/operatorstack/yield?style=flat-square" /></a>
</p>

<p align="center">
  <a href="https://yield.operatorstack.systems/">Website</a> ·
  <a href="https://yield.operatorstack.systems/docs/">Documentation</a> ·
  <a href="https://pkg.go.dev/github.com/operatorstack/yield/sdk/yield">pkg.go.dev</a> ·
  <a href="https://github.com/operatorstack/yield">GitHub</a>
</p>

The Go module is `github.com/operatorstack/yield`. Import the SDK as
`github.com/operatorstack/yield/sdk/yield`. The installed command is `yskill`.

## Start with your coding agent

```bash
go run github.com/operatorstack/yield/cmd/yskill@latest bootstrap --language go
```

Review and confirm the plan. Restart your coding agent. Then ask it to create
a new skill workflow:

```text
Use Yield to create a tested skill workflow for releasing my package.
```

To convert an existing `SKILL.md`, ask:

```text
Use Yield to convert my existing release SKILL.md into a tested skill workflow.
```

## Advanced: build manually

### 1. Install Yield

Yield supports Go on macOS, Linux, and Windows. Install the public command:

```bash
go install github.com/operatorstack/yield/cmd/yskill@latest
yskill --version
```

Go downloads the tagged module through
[`proxy.golang.org`](https://proxy.golang.org/). You do not need a separate
registry account or private package source.

### 2. Create the workflow

Create a Go workflow inside your repository:

```bash
yskill init skills/investigate \
  --language go \
  --description "Collect failure evidence, test hypotheses, and report the cause."
```

Replace `skills/investigate/main.go` with this tested workflow:

<!-- go-example:start -->
```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/operatorstack/yield/sdk/yield"
)

type hypothesis struct {
	ID              string `json:"id"`
	Statement       string `json:"statement"`
	DisproveCommand string `json:"disprove_command"`
}

type assessment struct {
	Refuted     bool   `json:"refuted"`
	CausalChain string `json:"causal_chain"`
}

const hypothesesSchema = `{
  "type": "object",
  "required": ["hypotheses"],
  "properties": {
    "hypotheses": {
      "type": "array",
      "minItems": 3,
      "items": {
        "type": "object",
        "required": ["id", "statement", "disprove_command"],
        "properties": {
          "id": {"type": "string"},
          "statement": {"type": "string"},
          "disprove_command": {"type": "string"}
        }
      }
    }
  }
}`

const assessmentSchema = `{
  "type": "object",
  "required": ["refuted"],
  "properties": {
    "refuted": {"type": "boolean"},
    "causal_chain": {"type": "string"}
  }
}`

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		evidence := ctx.AgentTask("collect-evidence",
			"Collect the observable evidence for the failure under investigation: error output, logs, recent changes. Return {\"observations\": [string]}.",
			nil, json.RawMessage(`{"type":"object","required":["observations"],"properties":{"observations":{"type":"array","items":{"type":"string"}}}}`))

		raw := ctx.AgentTask("form-hypotheses",
			"Produce at least three hypotheses explaining the evidence, ordered cheapest-to-disprove first. Each carries a shell command whose failure would disprove it.",
			json.RawMessage(evidence), json.RawMessage(hypothesesSchema))
		var hs struct {
			Hypotheses []hypothesis `json:"hypotheses"`
		}
		if err := json.Unmarshal(raw, &hs); err != nil {
			return yield.Outcome{}, err
		}

		failures := 0
		for _, h := range hs.Hypotheses {
			if failures >= 3 {
				break
			}
			result := ctx.RunCommand("probe-"+h.ID, h.DisproveCommand, 300)
			assessRaw := ctx.AgentTask("assess-"+h.ID,
				fmt.Sprintf("Hypothesis %q: %s. Given the probe result, is it refuted? If it survives, state the causal chain from root cause to observed failure.", h.ID, h.Statement),
				map[string]any{"hypothesis": h, "probe": result},
				json.RawMessage(assessmentSchema))
			var a assessment
			if err := json.Unmarshal(assessRaw, &a); err != nil {
				return yield.Outcome{}, err
			}
			if a.Refuted {
				failures++
				continue
			}
			ctx.Require(a.CausalChain != "", "the surviving hypothesis states a causal chain", a)
			return ctx.Complete(map[string]any{
				"hypothesis":   h,
				"causal_chain": a.CausalChain,
				"probe_exit":   result.ExitCode,
			})
		}
		return yield.Outcome{}, ctx.Blocked(
			fmt.Sprintf("frontier reached: %d hypotheses refuted with %d failed attempts and none surviving — new evidence is needed, not more guessing", len(hs.Hypotheses), failures))
	})
}
```
<!-- go-example:end -->

The generated `go.mod` pins the public Yield module to the installed CLI
version. The generated `skill.json` runs the Go program.

### 3. Test the workflow

Use deterministic responses during tests. Save this as
`skills/investigate/fixtures/responses.json`:

<!-- go-fixture:start -->
```json
{
  "collect-evidence": {
    "observations": [
      "CI fails on ubuntu only with 'Text file busy' (exit 126)",
      "failure started after the hydrate step became concurrent",
      "macOS and windows runners are green"
    ]
  },
  "form-hypotheses": {
    "hypotheses": [
      {
        "id": "h1",
        "statement": "The runner image is missing the binary entirely",
        "disprove_command": "exit 1"
      },
      {
        "id": "h2",
        "statement": "Concurrent hydrate writes the binary while another process execs it (ETXTBSY)",
        "disprove_command": "true"
      },
      {
        "id": "h3",
        "statement": "A permissions regression strips the execute bit",
        "disprove_command": "true"
      }
    ]
  },
  "assess-h1": {
    "refuted": true
  },
  "assess-h2": {
    "refuted": false,
    "causal_chain": "concurrent hydrate holds the binary open for write -> exec of the same inode returns ETXTBSY -> shell reports exit 126 -> job fails only where hydrate and exec overlap (ubuntu)"
  }
}
```
<!-- go-fixture:end -->

Then test the workflow:

```bash
yskill doctor skills/investigate --test
```

Yield runs commands for real and supplies agent responses from the fixture. A
successful test reaches `completed` without leaving a run journal.

### 4. Register the skill

Registration lets installed coding agents discover the workflow:

```bash
yskill register skills/investigate
```

Select the verified agents explicitly when you do not want automatic
detection:

```bash
yskill register skills/investigate \
  --agent cursor,codex,claude-code
```

The generated adapters point back to `skills/investigate`. They do not copy the
workflow or install its dependencies again.

### 5. Run the skill

Start a new coding-agent session so it discovers the registered skill. Where
slash skills are supported, run:

```text
/investigate
```

Otherwise, ask the agent in plain language:

```text
Use the investigate skill to diagnose this failure.
```

The agent follows the adapter, starts the canonical Go workflow, and supplies
each required agent response.

## How Yield runs and resumes

1. Your Go function emits one typed operation.
2. Yield records the request and exits. It does not run a daemon.
3. The coding agent, user, or CLI supplies the result.
4. Yield replays the function from its journal until it reaches the next
   operation.

Replay must produce the same operation sequence. Yield reports divergence
instead of giving a recorded response to a different operation.

| Go primitive | Purpose |
|---|---|
| `ctx.RunCommand()` | Execute a command and record its exit code and output. |
| `ctx.AgentTask()` | Ask the coding agent for schema-valid JSON. |
| `ctx.AskUser()` | Request an explicit human decision. |
| `ctx.Require()` | Bind a required claim to recorded evidence. |
| `ctx.Blocked()` / `ctx.Refused()` | Stop honestly when work cannot or must not continue. |

See the [Go reference](https://pkg.go.dev/github.com/operatorstack/yield/sdk/yield),
[primitive guides](https://yield.operatorstack.systems/docs/primitives/), and
[CLI reference](https://github.com/operatorstack/yield/blob/main/docs/reference/cli.md)
for the complete contract.

## Guarantees and limits

Yield provides deterministic control flow, typed requests and responses,
persistent run state, replay with divergence detection, stale and duplicate
response rejection, and evidence-bound completion.

Schema validity is not truth. Yield cannot prove that a coding agent performed
only the requested work. `RunCommand` is different: the Yield CLI executes the
command, so its recorded exit code and output are observed facts.

Programs must remain deterministic between operations. Do not read clocks,
random values, environment variables, or changing files to choose the next
operation. Cross those boundaries through a Yield operation instead.

Yield is not a daemon, hosted runtime, workflow DSL, marketplace, coding-agent
replacement, or permission sandbox. Your operating system, repository, and
coding-agent permissions remain the security boundary.

## Coding-agent support

Yield verifies adapters for Cursor, Codex, and Claude Code. Registry-backed
project paths are available for other coding agents. See the
[agent setup guide](https://yield.operatorstack.systems/docs/agent-setup/).

## Source and support

- [Go API reference](https://pkg.go.dev/github.com/operatorstack/yield/sdk/yield)
- [Public documentation](https://yield.operatorstack.systems/docs/)
- [Go source](https://github.com/operatorstack/yield/tree/main/sdk/yield)
- [Working examples](https://github.com/operatorstack/yield/tree/main/examples)
- [Issues](https://github.com/operatorstack/yield/issues)
- [Security policy](https://github.com/operatorstack/yield/blob/main/SECURITY.md)

Yield is available under the [MIT License](https://github.com/operatorstack/yield/blob/main/LICENSE).
