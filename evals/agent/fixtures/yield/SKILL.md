---
name: release-package-yield
description: Run the code-controlled package release workflow.
---

Run `bin/yskill run skills/release` and follow every returned operation.

- For `agent_task`, perform its instruction and return schema-valid JSON.
- For `ask_user`, run `node bin/user-answer.mjs <request-id>` and use the
  returned value as the user's answer.
- For either kind, write a response file using the values in the current
  request envelope:

  ```json
  {"run_id":"<run-id>","sequence":1,"request_id":"<request-id>","status":"completed","result":{}}
  ```

  Replace `sequence` and `result` with the current request's sequence and the
  schema-valid result. Then run `bin/yskill resume <run-id> --response <file>
  --skill skills/release`.

Do not run workflow commands yourself. Yield runs them. Do not skip a request,
invent an answer, edit the workflow, or use test expectations. End with the
terminal status reported by Yield.
