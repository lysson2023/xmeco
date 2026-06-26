# AI Agent Instruction

**Before making any changes to this project, read [`agents.md`](./agents.md) in full.**

It defines:
- Tech stack baseline (locked versions — do NOT upgrade)
- Architecture and dependency direction (one-way only)
- Forbidden patterns (no `_` discarding errors, no SQL string concat, no untested new code)
- Naming conventions (file, package, variable, migration file names)
- Migration rules (add-only, never modify existing, use `NNN_english_desc.sql`)
- Test requirements (new Repository/Gateway/Service code MUST have tests)
- Cache strategy (when to use, how to implement, when NOT to cache)

Failure to follow `agents.md` will result in code rejection.
