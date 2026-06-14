# Open Code Review — Roadmap

> Concise developer-facing roadmap distilled from
> `docs/embedded-c-review-blueprint.zh-CN.md` (the long-form blueprint,
> 451 lines) and the actual state of the code on `main`.

## What this project is

A rule-driven code review platform targeted at **embedded C** projects,
forked from `alibaba/open-code-review` and layered on top of its
agent / LLM / toolset / viewer stack.

**Core asset** = the rule set and the rule model, not the analyzers.
**Execution means** = multiple backends (regex, call extraction, cppcheck,
and in the future tree-sitter / clang AST / CodeQL).
**Top layer** = AI for context explanation and false-positive review.

## What this project is NOT

- Not a static-analysis tool aggregator that just merges raw output.
- Not an AST reimplementation from scratch. We reuse mature open-source
  analyzers (cppcheck, clang-tidy, Clang SA, CodeQL, semgrep) as
  rule backends.
- Not a GUI product. The interaction surface is CLI, JSON, SARIF, HTML.

## Five capability layers

Every rule in this project is hosted by exactly one layer. Each
layer has a hard upper bound on the precision it can deliver.

| Layer | What it sees | What it cannot see | Today's status |
|---|---|---|---|
| **L1 — text/pattern** | raw tokens in a line | comments, strings, scopes, types, control flow | `RegexBackend` shipped |
| **L1.5 — call extraction** | identifier + `(` matching | function pointers, macro expansion, member calls | `CallBackend` shipped (regex-based; not a real AST) |
| **L2 — borrowed AST/DFA** | whatever the host tool can prove | cross-translation-unit paths, complex control flow | cppcheck adapter shipped; clang-tidy/CodeQL/semgrep adapters not yet |
| **L3 — real AST + CFG** | call graph, control flow, def-use | type system, deep aliasing, concurrency | **deferred** — needs tree-sitter or clang AST |
| **L4 — semantic / domain** | state machines, safety invariants, project-specific rules | automatic proof | **deferred** — needs CodeQL or custom DSL |

**Rule precision is bounded by the weakest layer that touches it.**
If a rule runs on `RegexBackend` (L1), it cannot be more accurate
than L1, no matter how clever its pattern. To raise a rule's
accuracy, the only honest path is to move it to a higher layer.

## Current state (as of v0.2.0)

- **17 rules** in `internal/rulesets/embedded_c/rules.json`
  (version 0.2.0).
- **3 backends live**: `RegexBackend` (L1), `CallBackend` (L1.5,
  regex-based), `CppcheckBackend` (L2/L3 borrowed).
- **cppcheck false-positive guards**: 5 rules have
  `disposition_overrides` to downgrade known-bad patterns
  (CANTRS.all, `inconclusive`, `Assuming`, `Member variable not
  initialized`, etc.). The first red-line regression test
  guarantees genuine `uninitvar` reports stay `blocking`.
- **22 fixture tests** under `internal/ruleengine/testdata/embedded_c/`,
  including a red-line test (`TestCppcheckBackendPreservesGenuineUninitvar`).
- **All 8 packages** with tests pass under `go test ./...`.

## Rule coverage (17 of 21 from blueprint §10)

| Blueprint rule | Shipped? | Backend | Notes |
|---|---|---|---|
| `embedded.memory.no_strcpy` | yes | regex | |
| `embedded.memory.no_sprintf` | yes | regex | |
| `embedded.loop.suspicious_infinite_wait` | yes | regex | `function_name_not_any=main` guard |
| `embedded.loop.require_watchdog_or_exit` | yes | regex | stricter than `suspicious_infinite_wait` |
| `embedded.isr.no_delay_call` | yes | call (role=isr) | |
| `embedded.isr.no_blocking_call` | yes | call (role=isr) | SemaphorePend / xQueueReceive / etc. |
| `embedded.isr.no_dynamic_allocation` | yes | call (role=isr) | malloc/kmalloc/etc. |
| `embedded.register.use_access_macro` | yes | regex (conservative) | only volatile pointer casts, not bare hex |
| `embedded.register.no_read_modify_write_on_status_clear` | yes | regex | covers `=`, `+=`, `-=`, `\|=`, `&=`, `^=` |
| `embedded.state.switch_requires_default` | yes (hint) | regex | flags `switch` opening only; precision awaits L2 |
| `misra_like.assignment_in_condition` | yes | regex | known limit: `a = b && c` patterns are tricky |
| `misra_like.macro_args_parenthesized` | yes (hint) | regex | single-line `#define` only |
| cppcheck.{redundantAssignment, unreadVariable, unusedVariable, uninitializedVariable, arrayIndexOutOfBounds} | yes | cppcheck | with `disposition_overrides` for known FPs |
| `embedded.pointer.null_check_before_deref` | **no** | needs L3 | regex can't trace null-check control flow |
| `embedded.buffer.bound_check_before_copy` | **no** | needs L3 | regex can't reason about size expressions |
| `embedded.volatile.shared_state_requires_volatile` | **no** | needs L3 | regex can't tell if a global is genuinely shared |
| `embedded.state.transition_requires_error_path` | **no** | needs L4 | state-machine analysis |
| `embedded.safety.shutdown_requires_pwm_disable` | **no** | needs L4 | requires inter-procedural reasoning |
| `embedded.fault.clear_requires_record` | **no** | needs L4 | error-propagation analysis |
| `misra_like.no_implicit_narrowing` | **no** | needs L2 | cppcheck can do it; not wired up yet |
| `misra_like.non_void_all_paths_return` | **no** | needs L2 | regex false-positive rate too high |
| `misra_like.no_unused_return_value` | **no** | needs L2 | requires call-site analysis |

**The 9 deferred rules are not "missing features" — they are
correctly placed at the layer that can actually express them. Adding
them at L1 would create a stream of false positives and erode trust
in the platform.**

## Five-phase rollout (from blueprint §11)

### Phase 1 — rule model + minimal engine   *(DONE)*

Rule JSON schema, Finding / Evidence model, regex backend, diff /
file input, JSON output, basic tests. See
`internal/ruleengine/{model,diff,suppression,normalize}.go`.

### Phase 2 — initial embedded C rule set   *(DONE, v0.2.0)*

17 rules, including ISR / loop / register / state / MISRA-inspired
categories, plus cppcheck adapter with false-positive guards.

### Phase 3 — AI review layer   *(NEXT)*

Wire `--ai` on by default. Use the LLM to label findings
`confirmed / needs_review / ignored`, and feed the label back into
`resolveFindingDisposition` so `needs_review` automatically drops
to `report_only`. Expected lift: removes the last 5% of human
review on borderline cppcheck findings.

### Phase 4 — AST + external backends   *(KEY UNLOCK)*

Replace regex-only `CallBackend` and `RegexBackend` for the
remaining rules with:

- **tree-sitter + C grammar** as the L2 backend (planned in
  blueprint §6, §8). The C grammar is small, well-maintained, and
  handles everything outside the C preprocessor.
- **CodeQL** as the L3/L4 backend. Rules become `.ql` files —
  exactly the "rules are the product" direction the blueprint
  calls for.
- **clang-tidy / Clang Static Analyzer** as an optional L2/L3
  adapter, similar in shape to `CppcheckBackend`.

This phase unblocks the 9 deferred rules.

### Phase 5 — CI + engineering hardening   *(MOSTLY DONE)*

GitHub Actions, GitLab CI examples, JSON / SARIF / HTML output,
inline suppression, rule disposition overrides, baseline
fingerprint. Outstanding:

- Per-project `fail-on` overrides (current default is a single
  global gate).
- Rule-level configuration so projects can override
  `cppcheck.uninitialized_variable.disposition` without forking
  the ruleset.

## Architectural migrations still pending

From blueprint §12, the explicit "current → target" mapping:

- `internal/staticanalysis/` is a **transitional implementation**.
  Blueprint §12 says: "后续建议逐步迁移至 `internal/ruleengine`".
  Concretely: every analyzer adapter that lives under
  `staticanalysis` (cppcheck, semgrep) should eventually have a
  sibling under `ruleengine/backends/` that participates in the
  rule-driven pipeline.
- The `RegexBackend` block-extraction logic for
  `block_not_any` (see `regex_backend.go:extractBlock`) is
  bracey-matching by line. It is correct for small blocks but
  will give wrong answers on multi-line macros / string literals
  spanning braces. Replace with tree-sitter once L2 lands.

## How to add a new rule

1. Pick the layer the rule belongs to. If it needs control-flow
   or type information, do not add it at L1.
2. Write a fixture in `internal/ruleengine/testdata/embedded_c/`
   that exhibits the problem, plus a corresponding test in
   `embedded_c_fixtures_test.go` that asserts the rule fires
   (positive) or doesn't fire (negative).
3. For rules with known false-positive patterns, add
   `disposition_overrides` rather than weakening the pattern.
4. Bump `version` in `rules.json` and add a fixture under
   `round2/` (or a new round folder for larger changes) so the
   test history stays readable.

## How to add a new backend

The contract is `internal/ruleengine/model.go:Backend`:

```go
type Backend interface {
    Name() string
    Supports(rule Rule) bool
    Analyze(req Request, rules []Rule) ([]Finding, error)
}
```

Implement it, register it in `rules_scan_cmd.go` alongside
`RegexBackend`, `CallBackend`, `CppcheckBackend`, and write a
fixture test that exercises the same XML / IR / output contract.

## How to read this project in 5 minutes

1. `README.md` — what it is and how to use it.
2. `docs/embedded-c-review-blueprint.zh-CN.md` — why it is
   shaped the way it is (the long version).
3. `internal/rulesets/embedded_c/rules.json` — the actual rules
   shipped today.
4. `internal/ruleengine/embedded_c_fixtures_test.go` — the
   tests that pin those rules' behaviour.
5. `ROADMAP.md` (this file) — where the project is going next.

## What success looks like

- **Short term**: a project author can drop the `ocr` CLI into a
  CI pipeline, run `ocr rules scan`, and the report has no obvious
  false positives blocking the pipeline.
- **Mid term**: the embedded C rule set is project-configurable;
  the AI layer is the default noise filter; CI integrations
  produce line-level review comments on PRs.
- **Long term**: a curated, project-portable embedded C rule
  library, with the rule definitions living in data files (`.ql`,
  JSON), not in Go source. Commercial tool reports (LDRA, PC-lint,
  Cppcheck Premium) import into the same Finding / Evidence model
  via adapters.
