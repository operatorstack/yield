# Agent Plugins and Yield

Agent Plugins and Yield solve different parts of the same delivery path:

- **Agent Plugins packages a capability.** Its manifest and fixed directories let
  compatible clients discover Agent Skills and MCP servers.
- **Yield governs execution.** A Yield program owns repeatable order, branches,
  checks, human decisions, saved state, and terminal outcomes.

In short: **Agent Plugins packages the capability. Yield governs the execution.**

## Current support boundary

Yield does not currently emit an Agent Plugins `plugin.json`, install plugins, or
load MCP servers. `yskill register` writes a small `SKILL.md` adapter into a coding
agent's project skill directory and keeps the canonical workflow beside the
project dependencies it uses.

That adapter alone is not proof that a package conforms to Agent Plugins. A
conformant plugin must satisfy the complete Agent Plugins manifest, containment,
component-discovery, and Agent Skills requirements. Yield does not claim Agent
Plugins v1 conformance in this release.

## Using the projects together

You can package skills however you like and use Yield for the repeatable workflow
behind them. Keep these boundaries explicit:

1. the plugin package owns discovery and portable package structure;
2. the Yield program owns workflow execution and verification;
3. the host owns installation, permissions, and the agent interaction surface.

A future plugin export should be treated as a separate, tested distribution
feature. It should not change Yield's execution contract.

See the [Agent Plugins specification](https://agent-plugins.org/specification) and
the [Agent Skills specification](https://agentskills.io/specification) for their
normative requirements.
