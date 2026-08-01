---
name: mcp-builder
description: Design and review a small MCP server around a defined use case.
---

# MCP builder

Turn the supplied use case into a minimal tool surface. For each tool, define a
specific name, typed inputs, bounded output, error behavior, and one realistic
example. Prefer fewer composable tools. Keep secrets out of arguments and make
destructive effects explicit.

During review, check schema clarity, transport errors, authentication boundaries,
pagination, idempotency, and whether evaluations cover success and failure paths.

Yield owns research approval, scaffold and test commands, evaluation thresholds,
saved artifacts, and completion.
