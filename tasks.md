# Implementation Tasks

## Phase 1: Package expansion infrastructure

### TASK-001: Add module-root-aware package expansion helper
**Status:** pending | **Attempts:** 0

**Description:** Add an internal helper that resolves the module root and expands Go package patterns (e.g., `./...`) using `go list` from the module root.

**Acceptance Criteria:**
- [ ] Helper returns a slice of package import paths for a pattern using `go list`.
- [ ] Helper runs `go list` relative to the module root (as determined by `go env GOMOD`).
- [ ] If `go env GOMOD` fails or returns empty, helper returns an error.
- [ ] If `go list` fails, helper returns an error.

**Files:** Modify: `internal/exec/exec.go`, Create/Modify: `internal/discovery/expand.go` (or similar), Create: `internal/discovery/expand_test.go`
**Dependencies:** None
**Traceability:** REQ-001, REQ-003, REQ-006, REQ-010, ARCH-001, ARCH-003, DES-001

---

## Phase 2: Baseline discovery integration

### TASK-002: Expand baseline packages before matching tests
**Status:** pending | **Attempts:** 0

**Description:** Integrate package expansion into baseline test discovery so `BaselineTestSpec.Package` patterns (including `./...`) expand to all matching packages for both pattern and exact-match modes.

**Acceptance Criteria:**
- [ ] When `TestPattern` is non-empty, pattern matching is applied to all expanded packages.
- [ ] When `TestPattern` is empty, all tests from all expanded packages are treated as baseline tests.
- [ ] Expanded packages are resolved relative to module root.
- [ ] Expansion errors are returned to the caller.

**Files:** Modify: `testredundancy.go`, `internal/discovery/discovery.go`
**Dependencies:** TASK-001
**Traceability:** REQ-001, REQ-002, REQ-004, REQ-005, REQ-007, REQ-009, ARCH-001, ARCH-002, ARCH-004, DES-001, DES-002

---

### TASK-003: Add tests for expansion and error behavior
**Status:** pending | **Attempts:** 0

**Description:** Add tests that validate baseline package expansion behavior and error handling.

**Acceptance Criteria:**
- [ ] `./...` expansion results in baseline matches across multiple packages (integration test using a temporary module fixture).
- [ ] Empty or invalid test pattern yields zero baseline matches without error.
- [ ] `go list` failure is surfaced as an error.
- [ ] Zero packages or packages without matching tests yield no baseline matches and no error.

**Files:** Create/Modify: `testredundancy_test.go` (or new fixture-based test files), `internal/discovery/*_test.go`
**Dependencies:** TASK-002
**Traceability:** REQ-001, REQ-002, REQ-003, REQ-006, REQ-007, REQ-008, REQ-009, REQ-010, REQ-011, ARCH-001, ARCH-003, DES-002

---

### TASK-004: Treat empty baseline pattern as no matches
**Status:** pending | **Attempts:** 0

**Description:** Ensure an empty `TestPattern` yields zero baseline matches (no error), aligning with REQ-008.

**Acceptance Criteria:**
- [ ] When `TestPattern` is empty, baseline selection yields zero baseline tests.
- [ ] No errors are raised for empty `TestPattern`.

**Files:** Modify: `testredundancy.go`, `testredundancy_test.go`
**Dependencies:** TASK-002
**Traceability:** REQ-008, ARCH-003, DES-002

---

## Dependency Graph

TASK-001
  -> TASK-002
    -> TASK-003
    -> TASK-004

## Parallelism Opportunities

None (sequential dependency chain).
