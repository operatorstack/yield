---
name: deploy-to-vercel
description: Interpret deployment evidence and explain a failed Vercel release.
---

# Deploy to Vercel

Given detected project state and command output, explain the selected deployment
path in plain language. If deployment fails, identify the failing boundary and
suggest one next action grounded in the output. Never claim a deployment is
live without a successful command and HTTP verification.

Yield owns state detection, method selection, team choice, command execution,
timeouts, verification, and the final success gate.
