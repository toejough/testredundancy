# Baseline Test Package Expansion - Product Specification

## Problem Statement
Baseline test discovery requires specifying individual packages, so patterns like `./...` do not expand and baseline tests are not found across all packages.

### Who is affected
Developers using the testredundancy library programmatically to configure baseline test discovery.

### Impact
Baseline tests are missed unless every package is listed explicitly, making configuration error-prone and time-consuming.

## Current State

### User Journey
- Developer constructs `testredundancy.Config` in code.
- Developer sets `BaselineTests` with `BaselineTestSpec{Package: "./...", TestPattern: "TestProperty_*"}`.
- Baseline test discovery does not find any matches because `./...` is treated as a literal package value.

### Pain Points
- `./...` does not expand to all packages.
- Must enumerate packages manually to get baseline matches.

### Constraints
- Library/programmatic usage only (no CLI/config changes required).

## Desired Future State

### Success Criteria
- **SC-01 (REQ-001):** Given `BaselineTests` uses `Package: "./..."`, baseline test discovery expands the package pattern and searches all matching packages.
- **SC-02 (REQ-002):** Given `TestPattern: "TestProperty_*"`, baseline test discovery matches all tests in the expanded packages whose names start with `TestProperty_`.
- **SC-03 (REQ-003):** If no packages or tests match, the result is simply no matches and no errors.

### User Stories (with explicit actions)
- **US-01 (REQ-004):** As a developer, I want to pass `Package: "./..."` in `BaselineTests` so that all packages are searched without listing each one.
  - **Action:** Developer sets `BaselineTestSpec{Package: "./...", TestPattern: "TestProperty_*"}` in code.
  - **Result:** Baseline tests are discovered across all packages that match `./...`.

### Acceptance Criteria (Given/When/Then format REQUIRED)
- **AC-01 (REQ-005):**
  - Given: `BaselineTests` includes `Package: "./..."` and `TestPattern: "TestProperty_*"`
  - When: Developer runs baseline test discovery
  - Then: All packages matched by `./...` are searched and tests starting with `TestProperty_` are included

- **AC-02 (REQ-006):**
  - Given: `./...` expands to zero packages
  - When: Developer runs baseline test discovery
  - Then: No matches are returned and no error is raised

- **AC-03 (REQ-007):**
  - Given: A package has no tests matching the pattern
  - When: Developer runs baseline test discovery
  - Then: That package contributes no matches and processing continues

- **AC-04 (REQ-008):**
  - Given: The test pattern is empty or invalid
  - When: Developer runs baseline test discovery
  - Then: No matches are returned and no error is raised

## Edge Cases

### Error Scenarios
- **REQ-009:** Package expansion yields a set including packages with no tests; those packages contribute no matches and do not cause errors.

### Boundary Conditions
- **REQ-010:** Package expansion yields zero packages; the overall result is zero matches without errors.

### Invariants
- **INV-01 (REQ-011):** Baseline test discovery must not fail solely due to lack of matches.

## Solution Guidance

### Approaches to Consider
Use `go list` to expand `./...` into matching packages.

### Approaches to Avoid
None specified.

### Constraints
Programmatic/library behavior only; no CLI/config changes required.

### References
- Example usage: `dev/targets.go` in consumer repos (targ, imptest).

## Open Questions
None.
