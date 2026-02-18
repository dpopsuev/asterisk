# Contract — Go testing preferences (standard vs framework, rules)

**Status:** complete  
**Goal:** Define when to use standard library `testing` vs a testing framework in Asterik Go code, and capture preferences as rules so agents and contributors apply them consistently. Research completed; findings in `notes/go-testing-frameworks-research.mdc`.

## Contract rules

- Global rules only. Do not change existing test behavior; document and apply going forward.
- Rule lives under `.cursor/rules/` and is referenced from this contract.

## Context

- **Current state:** All tests use stdlib `testing` only; no third-party test deps in `go.mod`. See `internal/preinvest`, `internal/investigate`, `internal/postinvest`, `internal/wiring`.
- **Methodology:** BDD/TDD, red–green–blue. `rules/project-standards.mdc`; contracts mock-pre-investigation through mock-wiring.
- **Need:** Clear guidance for when to add a framework (e.g. testify, ginkgo) vs stay standard; naming and structure conventions.

## Execution strategy

1. Define criteria: standard vs framework (e.g. unit/table-driven → standard; BDD scenarios with many steps → consider framework).
2. Write rule file with preferences and examples.
3. Add rule to index; link from contract. No change to existing tests unless explicitly in scope.

## Tasks

- [x] **Define preferences** — When standard: unit tests, table-driven, small helpers, no external process. When framework: optional for BDD-style scenarios, integration/e2e with setup teardown, or when team explicitly adopts one.
- [x] **Create rule** — Add `rules/go-test-conventions.mdc` with: prefer standard by default; when to consider a framework; test naming (TestXxx); helpers (t.Helper()); fixtures; no test deps in go.mod unless decided.
- [x] **Index and link** — Update `rules/index.mdc`; add contract to `contracts/index.mdc`; note in contract Notes.
- [x] **Validate** — Rule is findable and unambiguous; existing tests remain standard-only.

## Research (framework and use cases)

- [x] **Research Go testing frameworks online** — Look up current options: testify (assert, require, suite), ginkgo/gomega (BDD), go-cmp, quick, httptest, etc. Note: purpose, typical use cases, pros/cons, adoption.
- [x] **Document use cases** — For each framework or pattern: when it shines (e.g. assertion-heavy tests, BDD scenarios, table-driven with deep equality, API tests). Add findings to this contract (Notes or a new “Research findings” section) or to a note under `notes/`.
- [x] **Update rule if needed** — After research: refine `rules/go-test-conventions.mdc` with concrete framework names, use-case mapping, and “consider X when Y” so the rule stays accurate. Re-validate (tests still standard unless adoption is decided).

## Research findings (summary)

- **Stdlib:** Default; zero deps, good for unit/table-driven and BDD-in-comments.
- **Testify:** Assertions, suites, mocks; consider when team adopts; argument order and deps are trade-offs.
- **Ginkgo + Gomega:** BDD DSL and matchers; consider for formal BDD or integration/async; steeper curve.
- **go-cmp:** Deep equality and `cmp.Diff`; small test-only dep; good when comparing complex structs without adopting a full framework.
- **httptest:** Stdlib; use for HTTP tests. **quick:** Stdlib; property-style tests.
- Full write-up: `notes/go-testing-frameworks-research.mdc`.
- **Ginkgo adopted (hybrid):** Team decision to adopt Ginkgo + Gomega for BDD/integration specs; use stdlib for simple unit tests. Rule updated (`rules/go-test-conventions.mdc`); added `internal/wiring/suite_test.go` and `internal/wiring/run_spec.go`. Not exclusive: hybrid approach.

## Acceptance criteria

- **Given** a developer or agent adding or changing Go tests in Asterik,
- **When** they check the rule,
- **Then** they know: (1) default = standard library `testing`; (2) when a framework is acceptable or recommended; (3) naming and structure conventions (TestXxx, t.Helper(), table-driven, fixture paths).
- **And** the rule is linked from the rules index and this contract.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-09 — Contract created; rule `rules/go-test-conventions.mdc` added with standard-by-default, when-to-use-framework, and conventions. Index updated.
- 2026-02-09 — Validate: rule in index and linked from contract; `go test ./...` passes; no test framework deps in go.mod. Contract complete.
- 2026-02-09 — Revived: status → active. Added Research section: research Go testing frameworks online, document use cases, update rule if needed.
- 2026-02-09 — Contract run: Researched testify, ginkgo/gomega, go-cmp, quick, httptest. Wrote `notes/go-testing-frameworks-research.mdc`. Updated `rules/go-test-conventions.mdc` with framework table and “consider X when Y”; added go-cmp and httptest to summary. Re-validated: `go test ./...` passes; no new deps. Research tasks complete.
- 2026-02-09 — Ginkgo adopted (hybrid): added ginkgo/v2 + gomega to go.mod; created internal/wiring/suite_test.go and run_spec.go; updated rule to Ginkgo for BDD/integration and stdlib for simple unit; documented in Research findings and rule.
