# Technical Architecture: testredundancy Baseline Package Expansion

## 1. Overview
This change updates baseline test discovery to expand Go package patterns (e.g., `./...`) while keeping the public API unchanged. The system remains a library-only workflow that interprets existing `BaselineTestSpec` fields during discovery. Package expansion will be resolved relative to the module root using `go list`, and errors from package discovery are surfaced to callers.

## 2. Requirements Traceability

| Requirement | Technical Implication | Addressed By |
|-------------|------------------------|--------------|
| REQ-001 | Expand baseline package patterns like `./...` | ARCH-001 (Package expansion via go list) |
| REQ-002 | Match tests by name pattern across expanded packages | ARCH-002 (Per-package test discovery) |
| REQ-003 | No matches are not errors | ARCH-003 (No-match handling) |
| REQ-004 | API remains unchanged | ARCH-004 (No API changes) |
| REQ-005 | Expand packages before searching | ARCH-001 |
| REQ-006 | Zero packages => no matches | ARCH-003 |
| REQ-007 | Package with no matches => no matches | ARCH-003 |
| REQ-008 | Empty/invalid test pattern => no matches | ARCH-003 |
| REQ-009 | Packages without tests do not error | ARCH-003 |
| REQ-010 | Zero packages => no matches | ARCH-003 |
| REQ-011 | Lack of matches never fails discovery | ARCH-003 |

## 3. Technology Stack

| Layer | Choice | Rationale | ARCH ID |
|-------|--------|-----------|---------|
| Language | Go (existing) | Consistency with library | ARCH-004 |
| Package expansion | `go list` | Standard, module-aware, aligns with `go test` semantics | ARCH-001 |

## 4. Architecture

### ARCH-001: Package expansion via go list
Traces to: REQ-001, REQ-005, DES-001

- Use `go list` to expand `BaselineTestSpec.Package` when it contains Go package patterns (e.g., `./...`).
- Resolve patterns relative to the module root so behavior matches `go test ./...` regardless of caller working directory.
- If `go list` fails, return the error to the caller (do not treat as no matches).

### ARCH-002: Per-package test discovery
Traces to: REQ-002, DES-002

- For each expanded package, enumerate tests and apply `TestPattern` matching.
- Aggregate matches across all expanded packages.

### ARCH-003: No-match handling semantics
Traces to: REQ-003, REQ-006, REQ-007, REQ-008, REQ-009, REQ-010, REQ-011, DES-002

- If expansion yields zero packages, return a successful result with zero baseline matches.
- If a package has no tests or no tests matching the pattern, contribute zero matches without error.
- Empty or invalid test patterns yield zero matches (no error).
- Only package expansion failures (e.g., `go list` error) are surfaced as errors.

### ARCH-004: Public API stability
Traces to: REQ-004, DES-001

- No new public types, fields, or flags.
- Behavior change is internal to baseline test discovery.

## 5. Data Models

No new data models required.

## 6. Service Interfaces

No new public service interfaces required.

## 7. File Structure

No new top-level directories expected. Implementation will live in existing test discovery logic.

## 8. Technology Decisions

### Decisions Made
| Decision | Choice | Alternatives Considered | Rationale | ARCH ID |
|----------|--------|--------------------------|-----------|---------|
| Package expansion | `go list` | Filesystem walk, manual module parsing | Standard tooling, module-aware | ARCH-001 |
| Error handling | Return `go list` errors | Swallow as no matches | User wants visibility of unexpected failures | ARCH-003 |

### Patterns Used
| Pattern | Where | Why | ARCH ID |
|---------|-------|-----|---------|
| Internal behavior change | Baseline discovery | Preserve API surface | ARCH-004 |

## 9. Error Handling

- `go list` errors are returned to the caller.
- Lack of matches does not error.

## 10. Testing Strategy

- Add tests for package expansion with `./...` resolving relative to module root.
- Add tests for zero packages, empty pattern, and per-package no-match behavior.
- Add tests for surfacing `go list` errors.

## 11. Open Questions

None.
