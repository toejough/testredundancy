# Design Specification (API Surface)

## Overview
This change is a library-only behavior update. No UI, screens, or visual components are required. The design focuses on API interpretation and developer interaction with existing fields.

## API Surface

### DES-001: Baseline test package pattern interpretation
Traces to: REQ-001, REQ-004, REQ-005

- Keep the existing `testredundancy.Config` and `BaselineTestSpec` API unchanged.
- Interpret `BaselineTestSpec.Package` as a pattern that may include Go package expansion semantics (e.g., `./...`).
- Expansion occurs internally when discovering baseline tests; callers continue to pass the same fields as today.

### DES-002: Baseline test discovery behavior outcomes
Traces to: REQ-002, REQ-003, REQ-006, REQ-007, REQ-008, REQ-009, REQ-010, REQ-011

- If expansion yields zero packages or a package yields zero matching tests, the result is simply no baseline matches.
- Empty or invalid test patterns yield no matches and do not error.
- Lack of matches must not cause discovery to fail.

## Non-Goals
- No new CLI flags or configuration files.
- No new public types or functions required.

## Node ID Reference
No visual assets were created for this change.
