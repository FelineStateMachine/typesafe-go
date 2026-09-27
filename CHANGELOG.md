# Changelog

All notable changes to this project are documented here.

## [Unreleased]

No changes yet.

## [0.2.0] - 2026-09-26

Local request validation now matches the limits enforced by the hosted API,
so requests the service would always reject fail before any network call.

- Added `MaxChoiceOptions` (255) and `MaxScoreLevels` (10); `Validate` rejects
  Choice and Score questions that exceed them (the API returns HTTP 400).
- `Validate` rejects a null `State`, including a nil pointer (the API returns
  HTTP 422).
- `Validate` rejects null Score levels (the API returns HTTP 422).
- `Validate` rejects Noul questions with neither instructions nor a non-null
  criterion (the API returns HTTP 400).

## [0.1.0] - 2026-09-17

Initial public release of the unofficial, community-maintained Go SDK.

- Added a standard-library-only client for `SystemOne` and model listing.
- Added typed Noul, Choice, and Score questions and answers.
- Added context-aware timeouts, retry policy, request IDs, and typed errors.
- Added examples and compile-checked documentation.

[Unreleased]: https://github.com/FelineStateMachine/typesafe-go/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/FelineStateMachine/typesafe-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/FelineStateMachine/typesafe-go/releases/tag/v0.1.0
