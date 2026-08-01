---
name: agentic-actions-auditor
description: Audit AI-enabled CI workflows for concrete security risks.
---

# Agentic actions auditor

Inspect the supplied workflow files and repository context. Trace untrusted input
to agent prompts, tools, credentials, write permissions, network access, and
mutable dependencies. Report only findings with a concrete attack path.

Each finding must include severity, workflow and line, source, capability reached,
impact, evidence, and remediation. Distinguish exploitable paths from hardening
advice. State coverage gaps explicitly.

Yield owns file discovery, scope, evidence capture, required fields, report
generation, and completion.
