# 2. Compile themes as validated packages

Date: 2026-09-01

## Status

Accepted.

## Context and Problem Statement

The booking service must be reusable across brands while the first deployment uses the Nerds Who Fish identity.
Themes affect public trust and accessibility, so a theme must not be able to bypass the application's interaction and security boundaries.

## Considered Options

1. Compile validated theme packages into the application
2. Store arbitrary administrator-supplied CSS in Firestore
3. Fork the frontend for every brand

## Decision Outcome

Define a versioned theme manifest with semantic color, typography, shape, copy, and asset fields. Compile known manifests and assets into the frontend, select one by deployment configuration, and keep booking behavior and component structure shared. A theme may change presentation and approved copy but cannot inject scripts, HTML, or arbitrary CSS.

## Consequences

### Good

- New brands reuse one tested booking flow
- Themes remain reviewable, cacheable, and compatible with accessibility tests
- No stored CSS or script becomes a cross-site scripting boundary

### Bad

- Adding a theme requires a build and release
- The manifest cannot express every possible redesign without evolving its versioned contract
- Component-level variation must be designed explicitly instead of patched with arbitrary selectors

### Rejected because

- Stored arbitrary CSS is flexible but creates injection, upgrade, and accessibility failure modes that are difficult to bound.
- Per-brand forks immediately duplicate booking behavior and guarantee drift in security and calendar correctness.
