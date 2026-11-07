# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

---

# SafeShell project specifics

Go module `safeshell` lives in `safeshell/` (git root is this folder). See `README.md` and `doc/architecture.md`.

- **Check before commit:** from `safeshell/`: `make check`; for sandbox/hook/daemon changes also `make build && make integration`. `SS_SWEEP=1 go test ./test/integration -run Sweep` runs all labelled commands through the real binary.
- **Architecture rules:** `domain/classify/policy/report` stay pure (no I/O). `engine` talks to adapters only through consumer-side ports; `cmd/ss` is the only composition root. Effects and the AI can only raise a risk level, never lower it.
- **Classifier changes:** update `eval/corpus` (dev) and keep `eval/heldout` independent; `TestHeldoutReport` enforces accuracy floors.
- **AI changes:** `ss ai bench` (43 requests, fixed `eval.BenchEnv`) measures accuracy; cloud results vary run to run (about 95-100% for Gemini flash-lite).
- **Secrets:** never write API keys to the repo, config, tests or logs. Live tests pass keys via env vars only (`GEMINI_API_KEY` etc.).
- **Commits:** short conventional messages, no co-author lines, fewer bigger commits, never push unless asked. Don't rewrite history without approval.
- **Releases:** tag `vX.Y.Z` and push; `.github/workflows/release.yml` publishes. Version comes from `-X main.version`.
- **Don't** run downloaded binaries (llama-server) or bypass permission denials without the user's approval.
