# Contributing

Issues and pull requests are welcome. Please describe the user-visible
problem, keep changes focused, and include tests for behavior that can be
verified without the hosted API.

The project supports Go 1.27 and later and intentionally has no runtime
dependencies. Before opening a pull request, run:

```sh
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
```

Do not put API keys in tests, examples, issues, or commits. Live API checks
must be explicit and opt in with `TYPESAFE_LIVE_TEST=1`; the default test suite
must remain offline and deterministic.

Changes to the public API should explain compatibility and migration impact.
Keep wire-format details aligned with the TypeSafe API documentation, while
remembering that this repository is an independent community project and is
not maintained by TypeSafe.
